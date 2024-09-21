package frankenphp

// #include <stdlib.h>
// #include "frankenphp.h"
import "C"
import (
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	workersRequestChans sync.Map // map[fileName]chan *http.Request
	workersReadyWG      sync.WaitGroup
	workerShutdownWG    sync.WaitGroup
	workersAreReady     atomic.Bool
	workersDone         chan interface{}
)

// TODO: start all the worker in parallell to reduce the boot time
func initWorkers(opt []workerOpt) error {
	workersDone = make(chan interface{})
	for _, w := range opt {
		if err := startWorkers(w.fileName, w.num, w.env); err != nil {
			return err
		}
	}

	return nil
}

func startWorkers(fileName string, nbWorkers int, env PreparedEnv) error {
	absFileName, err := filepath.Abs(fileName)
	if err != nil {
		return fmt.Errorf("workers %q: %w", fileName, err)
	}

	if _, ok := workersRequestChans.Load(absFileName); ok {
		return fmt.Errorf("workers %q: already started", absFileName)
	}

	workersRequestChans.Store(absFileName, make(chan *http.Request))
	shutdownWG.Add(nbWorkers)
	workerShutdownWG.Add(nbWorkers)
	workersAreReady.Store(false)
	workersReadyWG.Add(nbWorkers)

	var (
		m    sync.RWMutex
		errs []error
	)

	if env == nil {
		env = make(PreparedEnv, 1)
	}

	env["FRANKENPHP_WORKER\x00"] = "1"

	l := getLogger()
	for i := 0; i < nbWorkers; i++ {
		go func() {
			defer shutdownWG.Done()
			defer workerShutdownWG.Done()
			for {
				// Create main dummy request
				r, err := http.NewRequest(http.MethodGet, filepath.Base(absFileName), nil)
				if err != nil {
					panic(err)
				}
				r, err = NewRequestWithContext(
					r,
					WithRequestDocumentRoot(filepath.Dir(absFileName), false),
					WithRequestPreparedEnv(env),
				)
				if err != nil {
					panic(err)
				}

				if c := l.Check(zapcore.DebugLevel, "starting"); c != nil {
					c.Write(zap.String("worker", absFileName), zap.Int("num", nbWorkers))
				}

				if err := ServeHTTP(nil, r); err != nil {
					panic(err)
				}

				fc := r.Context().Value(contextKey).(*FrankenPHPContext)

				// TODO: make the max restart configurable
				if _, ok := workersRequestChans.Load(absFileName); ok {
					if fc.exitStatus == 0 {
						if c := l.Check(zapcore.InfoLevel, "restarting"); c != nil {
							c.Write(zap.String("worker", absFileName))
						}
					} else {
						// we will wait a few milliseconds to not overwhelm the logger in case of repeated unexpected terminations
						time.Sleep(50 * time.Millisecond)
						if c := l.Check(zapcore.ErrorLevel, "unexpected termination, restarting"); c != nil {
							c.Write(zap.String("worker", absFileName), zap.Int("exit_status", int(fc.exitStatus)))
						}
					}
				} else {
					break
				}
			}

			// TODO: check if the termination is expected
			if c := l.Check(zapcore.DebugLevel, "terminated"); c != nil {
				c.Write(zap.String("worker", absFileName))
			}
		}()
	}

	workersReadyWG.Wait()
	workersAreReady.Store(true)
	m.Lock()
	defer m.Unlock()

	if len(errs) == 0 {
		return nil
	}

	return fmt.Errorf("workers %q: error while starting: %w", fileName, errors.Join(errs...))
}

func stopWorkers() {
	workersRequestChans.Range(func(k, v any) bool {
		workersRequestChans.Delete(k)

		return true
	})
	close(workersDone)
}

func drainWorkers() {
	stopWorkers()
	workerShutdownWG.Wait()
}

func restartWorkers(workerOpts []workerOpt) {
	drainWorkers()
	if err := initWorkers(workerOpts); err != nil {
		logger.Error("failed to restart workers when watching files")
		panic(err)
	}
	logger.Info("workers restarted successfully")
}

//export go_frankenphp_worker_ready
func go_frankenphp_worker_ready() {
	if !workersAreReady.Load() {
		workersReadyWG.Done()
	}
}

//export go_frankenphp_worker_handle_request_start
func go_frankenphp_worker_handle_request_start(threadId int) C.bool {
	thread := getPHPThread(threadId)
	mainRequest := thread.getMainRequest()
	fc := mainRequest.Context().Value(contextKey).(*FrankenPHPContext)

	v, ok := workersRequestChans.Load(fc.scriptFilename)
	if !ok {
		// Probably shutting down
		return C.bool(false)
	}

	rc := v.(chan *http.Request)
	l := getLogger()

	if c := l.Check(zapcore.DebugLevel, "waiting for request"); c != nil {
		c.Write(zap.String("worker", fc.scriptFilename))
	}

	var r *http.Request
	select {
	case <-workersDone:
		if c := l.Check(zapcore.DebugLevel, "shutting down"); c != nil {
			c.Write(zap.String("worker", fc.scriptFilename))
		}
		// TODO: should opcache_reset be conditional?
		resetOpCache()

		return C.bool(false)
	case r = <-rc:
	}

	thread.setWorkerRequest(r)

	if c := l.Check(zapcore.DebugLevel, "request handling started"); c != nil {
		c.Write(zap.String("worker", fc.scriptFilename), zap.String("url", r.RequestURI))
	}

	if err := updateServerContext(r, false, true); err != nil {
		// Unexpected error
		if c := l.Check(zapcore.DebugLevel, "unexpected error"); c != nil {
			c.Write(zap.String("worker", fc.scriptFilename), zap.String("url", r.RequestURI), zap.Error(err))
		}

		return C.bool(false)
	}
	return C.bool(true)
}

//export go_frankenphp_finish_request
func go_frankenphp_finish_request(threadId int, isWorkerRequest bool) {
	thread := getPHPThread(threadId)
	r := thread.getActiveRequest()
	fc := r.Context().Value(contextKey).(*FrankenPHPContext)

	if isWorkerRequest {
		thread.setWorkerRequest(nil)
	}

	maybeCloseContext(fc)

	if c := fc.logger.Check(zapcore.DebugLevel, "request handling finished"); c != nil {
		var fields []zap.Field
		if isWorkerRequest {
			fields = append(fields, zap.String("worker", fc.scriptFilename), zap.String("url", r.RequestURI))
		} else {
			fields = append(fields, zap.String("url", r.RequestURI))
		}

		c.Write(fields...)
	}
}

func resetOpCache() {
	success := C.frankenphp_execute_php_function(C.CString("opcache_reset"))
	if success == 1 {
		logger.Debug("opcache_reset successful")
	} else {
		logger.Error("opcache_reset failed")
	}
}
