package frankenphp

// #include <stdlib.h>
// #include "frankenphp.h"
import "C"
import (
	"fmt"
	"net/http"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dunglas/frankenphp/watcher"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type worker struct{
	fileName string
	num int
	env PreparedEnv
	requestChan chan *http.Request
}

var (
	workersReadyWG      sync.WaitGroup
	workerShutdownWG    sync.WaitGroup
	workersAreReady     atomic.Bool
	workersAreDone      atomic.Bool
	workersDone         chan interface{}
	workers 		    map[string]*worker = make(map[string]*worker)
)

func initWorkers(opt []workerOpt) error {
	workersAreReady.Store(false)
	workersAreDone.Store(false)
	workersDone = make(chan interface{})

	for _, o := range opt {
		worker, err := newWorker(o)
		if err != nil {
			return err
		}
		workersReadyWG.Add(worker.num)
		go worker.startWorkerThreads()
	}

	workersReadyWG.Wait()
    workersAreReady.Store(true)

	return nil
}

func newWorker(o workerOpt) (*worker, error) {
	absFileName, err := filepath.Abs(o.fileName)
    if err != nil {
        return nil, fmt.Errorf("worker filename is invalid %q: %w", o.fileName, err)
    }

	// if the worker already exists, return it
	// it's necessary since we don't want to destroy the channels when restarting on file changes
	if w, ok := workers[absFileName]; ok {
        return w, nil
    }

	if o.env == nil {
        o.env = make(PreparedEnv, 1)
    }

	o.env["FRANKENPHP_WORKER\x00"] = "1"
	w := &worker{fileName: absFileName, num: o.num, env: o.env, requestChan: make(chan *http.Request)}
    workers[absFileName] = w

    return w, nil
}

func (worker *worker) startWorkerThreads() {
	shutdownWG.Add(worker.num)
	workerShutdownWG.Add(worker.num)

	l := getLogger()
	for i := 0; i < worker.num; i++ {
		go func() {
			defer shutdownWG.Done()
			defer workerShutdownWG.Done()
			for {
				// Create main dummy request
				r, err := http.NewRequest(http.MethodGet, filepath.Base(worker.fileName), nil)

				metrics.StartWorker(worker.fileName)

				if err != nil {
					panic(err)
				}
				r, err = NewRequestWithContext(
					r,
					WithRequestDocumentRoot(filepath.Dir(worker.fileName), false),
					WithRequestPreparedEnv(worker.env),
				)
				if err != nil {
					panic(err)
				}

				if c := l.Check(zapcore.DebugLevel, "starting"); c != nil {
					c.Write(zap.String("worker", worker.fileName), zap.Int("num", worker.num))
				}

				if err := ServeHTTP(nil, r); err != nil {
					panic(err)
				}

				fc := r.Context().Value(contextKey).(*FrankenPHPContext)

				// TODO: make the max restart configurable
				if !workersAreDone.Load() {
					metrics.StopWorker(worker.fileName)
					if fc.exitStatus == 0 {
						if c := l.Check(zapcore.InfoLevel, "restarting"); c != nil {
							c.Write(zap.String("worker", worker.fileName))
						}
					} else {
						// we will wait a few milliseconds to not overwhelm the logger in case of repeated unexpected terminations
						time.Sleep(50 * time.Millisecond)
						if c := l.Check(zapcore.ErrorLevel, "unexpected termination, restarting"); c != nil {
							c.Write(zap.String("worker", worker.fileName), zap.Int("exit_status", int(fc.exitStatus)))
						}
					}
				} else {
					break
				}
			}

			// TODO: check if the termination is expected
			if c := l.Check(zapcore.DebugLevel, "terminated"); c != nil {
				c.Write(zap.String("worker", worker.fileName))
			}
		}()
	}
}

func stopWorkers() {
	workersAreDone.Store(true)
	close(workersDone)
}

func drainWorkers() {
	watcher.DrainWatcher()
	stopWorkers()
	workerShutdownWG.Wait()
	workers = make(map[string]*worker)
}

func restartWorkersOnFileChanges(workerOpts []workerOpt) error {
	directoriesToWatch := []string{}
	for _, w := range workerOpts {
		directoriesToWatch = append(directoriesToWatch, w.watch...)
	}
	restartWorkers := func() {
		restartWorkers(workerOpts)
	}
	if err := watcher.InitWatcher(directoriesToWatch, restartWorkers, getLogger()); err != nil {
		return err
	}

	return nil
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

func assignThreadToWorker(thread *PHPThread) {
	mainRequest := thread.getMainRequest()
	fc := mainRequest.Context().Value(contextKey).(*FrankenPHPContext)
	worker, ok := workers[fc.scriptFilename]
    if !ok {
        panic("worker not found for script: "+fc.scriptFilename)
    }
    thread.worker = worker
    if !workersAreReady.Load() {
        workersReadyWG.Done()
    }
    // TODO: we can also store all threads assigned to the worker if needed
}

//export go_frankenphp_worker_handle_request_start
func go_frankenphp_worker_handle_request_start(threadIndex int) C.bool {
	thread := getPHPThread(threadIndex)

	if workersAreDone.Load() {
		// shutting down
		return C.bool(false)
	}

	l := getLogger()

	if c := l.Check(zapcore.DebugLevel, "waiting for request"); c != nil {
		c.Write(zap.String("worker", thread.worker.fileName))
	}

	// we assign a worker to the thread if it doesn't have one already
    if(thread.worker == nil) {
        assignThreadToWorker(thread)
    }

	var r *http.Request
	select {
	case <-workersDone:
		if c := l.Check(zapcore.DebugLevel, "shutting down"); c != nil {
			c.Write(zap.String("worker", thread.worker.fileName))
		}
		executePHPFunction("opcache_reset")

		return C.bool(false)
	case r = <-thread.worker.requestChan:
	}

	thread.setWorkerRequest(r)

	if c := l.Check(zapcore.DebugLevel, "request handling started"); c != nil {
		c.Write(zap.String("worker", thread.worker.fileName), zap.String("url", r.RequestURI))
	}

	if err := updateServerContext(r, false, true); err != nil {
		// Unexpected error
		if c := l.Check(zapcore.DebugLevel, "unexpected error"); c != nil {
			c.Write(zap.String("worker", thread.worker.fileName), zap.String("url", r.RequestURI), zap.Error(err))
		}

		return C.bool(false)
	}
	return C.bool(true)
}

//export go_frankenphp_finish_request
func go_frankenphp_finish_request(threadIndex int, isWorkerRequest bool) {
	thread := getPHPThread(threadIndex)
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
