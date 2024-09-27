package frankenphp

// #include <stdlib.h>
// #include "frankenphp.h"
import "C"
import (
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"runtime/cgo"
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
	workersAreDone      atomic.Bool
	workersDone         chan interface{}
)

// TODO: start all the worker in parallell to reduce the boot time
func initWorkers(opt []workerOpt) error {
	workersDone = make(chan interface{})
	workersAreReady.Store(false)
	workersAreDone.Store(false)

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

	if _, ok := workersRequestChans.Load(absFileName); !ok {
		workersRequestChans.Store(absFileName, make(chan *http.Request))
	}

	shutdownWG.Add(nbWorkers)
	workerShutdownWG.Add(nbWorkers)
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

				metrics.StartWorker(absFileName)

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
				if fc.currentWorkerRequest != 0 {
					// Terminate the pending HTTP request handled by the worker
					maybeCloseContext(fc.currentWorkerRequest.Value().(*http.Request).Context().Value(contextKey).(*FrankenPHPContext))
					fc.currentWorkerRequest.Delete()
					fc.currentWorkerRequest = 0
				}

				// TODO: make the max restart configurable
				if !workersAreDone.Load() {
					metrics.StopWorker(absFileName)
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
	workersAreDone.Store(true)
	close(workersDone)
}

func drainWorkers() {
	stopWorkers()
	workerShutdownWG.Wait()
	workersRequestChans = sync.Map{}
}

func restartWorkers(workerOpts []workerOpt) {
	stopWorkers()
	workerShutdownWG.Wait()
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
func go_frankenphp_worker_handle_request_start(mrh C.uintptr_t) C.uintptr_t {
	mainRequest := cgo.Handle(mrh).Value().(*http.Request)
	fc := mainRequest.Context().Value(contextKey).(*FrankenPHPContext)

	v, ok := workersRequestChans.Load(fc.scriptFilename)
	if !ok {
		// Probably shutting down
		return 0
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
		executePhpFunction("opcache_reset")

		return 0
	case r = <-rc:
	}

	fc.currentWorkerRequest = cgo.NewHandle(r)
	r.Context().Value(handleKey).(*handleList).AddHandle(fc.currentWorkerRequest)

	if c := l.Check(zapcore.DebugLevel, "request handling started"); c != nil {
		c.Write(zap.String("worker", fc.scriptFilename), zap.String("url", r.RequestURI))
	}

	if err := updateServerContext(r, false, mrh); err != nil {
		// Unexpected error
		if c := l.Check(zapcore.DebugLevel, "unexpected error"); c != nil {
			c.Write(zap.String("worker", fc.scriptFilename), zap.String("url", r.RequestURI), zap.Error(err))
		}

		return 0
	}

	return C.uintptr_t(fc.currentWorkerRequest)
}

//export go_frankenphp_finish_request
func go_frankenphp_finish_request(mrh, rh C.uintptr_t, deleteHandle bool) {
	rHandle := cgo.Handle(rh)
	r := rHandle.Value().(*http.Request)
	fc := r.Context().Value(contextKey).(*FrankenPHPContext)

	if deleteHandle {
		r.Context().Value(handleKey).(*handleList).FreeAll()
		cgo.Handle(mrh).Value().(*http.Request).Context().Value(contextKey).(*FrankenPHPContext).currentWorkerRequest = 0
	}

	maybeCloseContext(fc)

	if c := fc.logger.Check(zapcore.DebugLevel, "request handling finished"); c != nil {
		var fields []zap.Field
		if mrh == 0 {
			fields = append(fields, zap.String("worker", fc.scriptFilename), zap.String("url", r.RequestURI))
		} else {
			fields = append(fields, zap.String("url", r.RequestURI))
		}

		c.Write(fields...)
	}
}
