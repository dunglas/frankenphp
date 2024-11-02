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

	"github.com/dunglas/frankenphp/internal/watcher"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type worker struct {
	fileName    string
	num         int
	env         PreparedEnv
	requestChan chan *http.Request
}

var (
	watcherIsEnabled bool
	workerShutdownWG sync.WaitGroup
	workersAreDone   atomic.Bool
	workersDone      chan interface{}
	workers          = make(map[string]*worker)
)

func initWorkers(opt []workerOpt) error {
	workersDone = make(chan interface{})
	workersAreDone.Store(false)

	for _, o := range opt {
		worker, err := newWorker(o)
		if err != nil {
			return err
		}
		for i := 0; i < worker.num; i++ {
			if err := startNewWorkerThread(worker); err != nil {
				return err
			}
		}
	}

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

func stopWorkers() {
	workersAreDone.Store(true)
	close(workersDone)
}

func drainWorkers() {
	watcher.DrainWatcher()
	watcherIsEnabled = false
	stopWorkers()
	workerShutdownWG.Wait()
	workers = make(map[string]*worker)
}

func restartWorkersOnFileChanges(workerOpts []workerOpt) error {
	var directoriesToWatch []string
	for _, w := range workerOpts {
		directoriesToWatch = append(directoriesToWatch, w.watch...)
	}
	watcherIsEnabled = len(directoriesToWatch) > 0
	if !watcherIsEnabled {
		return nil
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

//export go_before_worker_script
func go_before_worker_script(threadIndex C.uintptr_t) *C.char {
	thread := phpThreads[threadIndex]
	worker := thread.worker

	// if we are done, exit the loop that restarts the worker script
	if workersAreDone.Load() {
		return nil
	}

	// if we are restarting the worker, reset the exponential failure backoff
	thread.backoff.reset()
	metrics.StartWorker(worker.fileName)

	// Create a dummy request to set up the worker
	r, err := http.NewRequest(http.MethodGet, filepath.Base(worker.fileName), nil)
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

	if err := updateServerContext(r, true, false); err != nil {
		panic(err)
	}

	thread.mainRequest = r
	if c := logger.Check(zapcore.DebugLevel, "starting"); c != nil {
		c.Write(zap.String("worker", worker.fileName), zap.Int("num", worker.num))
	}

	return C.CString(worker.fileName)
}

//export go_after_worker_script
func go_after_worker_script(threadIndex C.uintptr_t, exitStatus C.int) {
	thread := phpThreads[threadIndex]
	fc := thread.mainRequest.Context().Value(contextKey).(*FrankenPHPContext)
	fc.exitStatus = exitStatus

	if fc.exitStatus < 0 {
		panic(ScriptExecutionError)
	}

	defer func() {
        maybeCloseContext(fc)
        thread.mainRequest = nil
        thread.Unpin()
    }()

	// on exit status 0 we just run the worker script again
	if fc.exitStatus == 0 {
		// TODO: make the max restart configurable
		metrics.StopWorker(thread.worker.fileName, StopReasonRestart)

		if c := logger.Check(zapcore.InfoLevel, "restarting"); c != nil {
			c.Write(zap.String("worker", thread.worker.fileName))
		}
		return
	}

	// on exit status 1 we apply an exponential backoff when restarting
	metrics.StopWorker(thread.worker.fileName, StopReasonCrash)
	thread.backoff.trigger(func(failureCount int) {
		// if we end up here, the worker has not been up for backoff*2
		// this is probably due to a syntax error or another fatal error
		if !watcherIsEnabled {
			panic(fmt.Errorf("workers %q: too many consecutive failures", thread.worker.fileName))
		}
		logger.Warn("many consecutive worker failures", zap.String("worker", thread.worker.fileName), zap.Int("failures", failureCount))
	})
}

//export go_frankenphp_worker_handle_request_start
func go_frankenphp_worker_handle_request_start(threadIndex C.uintptr_t) C.bool {
	thread := phpThreads[threadIndex]
	thread.setReadyForRequests()

	if c := logger.Check(zapcore.DebugLevel, "waiting for request"); c != nil {
		c.Write(zap.String("worker", thread.worker.fileName))
	}

	var r *http.Request
	select {
	case <-workersDone:
		if c := logger.Check(zapcore.DebugLevel, "shutting down"); c != nil {
			c.Write(zap.String("worker", thread.worker.fileName))
		}
		executePHPFunction("opcache_reset")

		return C.bool(false)
	case r = <-thread.worker.requestChan:
	}

	thread.workerRequest = r

	if c := logger.Check(zapcore.DebugLevel, "request handling started"); c != nil {
		c.Write(zap.String("worker", thread.worker.fileName), zap.String("url", r.RequestURI))
	}

	if err := updateServerContext(r, false, true); err != nil {
		// Unexpected error
		if c := logger.Check(zapcore.DebugLevel, "unexpected error"); c != nil {
			c.Write(zap.String("worker", thread.worker.fileName), zap.String("url", r.RequestURI), zap.Error(err))
		}

		return C.bool(false)
	}
	return C.bool(true)
}

//export go_frankenphp_finish_request
func go_frankenphp_finish_request(threadIndex C.uintptr_t, isWorkerRequest bool) {
	thread := phpThreads[threadIndex]
	r := thread.getActiveRequest()
	fc := r.Context().Value(contextKey).(*FrankenPHPContext)

	if isWorkerRequest {
		thread.workerRequest = nil
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

	thread.Unpin()
}
