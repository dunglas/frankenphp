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

func startNewWorkerThread(worker *worker) error {
	workerShutdownWG.Add(1)
	thread := getInactivePHPThread()
	thread.onStartup = func(thread *phpThread) {
		thread.worker = worker
		metrics.ReadyWorker(worker.fileName)
		thread.backoff = newExponentialBackoff()
	}
	thread.onWork = runWorkerScript
	thread.onShutdown = func(thread *phpThread) {
		thread.worker = nil
		workerShutdownWG.Done()
	}
	return thread.run()
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

func runWorkerScript(thread *phpThread) bool {
	// if workers are done, we stop the loop that runs the worker script
	if workersAreDone.Load() {
		return false
	}
	beforeWorkerScript(thread)
	exitStatus := executeScriptCGI(thread.worker.fileName)
	afterWorkerScript(thread, exitStatus)

	return true
}

func beforeWorkerScript(thread *phpThread) {
	worker := thread.worker

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
		c.Write(zap.String("worker", worker.fileName), zap.Int("thread", thread.threadIndex))
	}
}

func afterWorkerScript(thread *phpThread, exitStatus C.int) {
	fc := thread.mainRequest.Context().Value(contextKey).(*FrankenPHPContext)
	fc.exitStatus = exitStatus

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

	if c := logger.Check(zapcore.DebugLevel, "waiting for request"); c != nil {
		c.Write(zap.String("worker", thread.worker.fileName))
	}

	var r *http.Request
	select {
	case <-workersDone:
		if c := logger.Check(zapcore.DebugLevel, "shutting down"); c != nil {
			c.Write(zap.String("worker", thread.worker.fileName))
		}
		if !executePHPFunction("opcache_reset") {
			logger.Warn("opcache_reset failed")
		}

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

//export go_frankenphp_finish_worker_request
func go_frankenphp_finish_worker_request(threadIndex C.uintptr_t) {
	thread := phpThreads[threadIndex]
	r := thread.getActiveRequest()
	fc := r.Context().Value(contextKey).(*FrankenPHPContext)
	thread.workerRequest = nil

	maybeCloseContext(fc)

	if c := fc.logger.Check(zapcore.DebugLevel, "request handling finished"); c != nil {
		c.Write(zap.String("worker", fc.scriptFilename), zap.String("url", r.RequestURI))
	}

	thread.Unpin()
}

// when frankenphp_finish_request() is directly called from PHP
//
//export go_frankenphp_finish_request_manually
func go_frankenphp_finish_request_manually(threadIndex C.uintptr_t) {
	thread := phpThreads[threadIndex]
	r := thread.getActiveRequest()
	fc := r.Context().Value(contextKey).(*FrankenPHPContext)
	maybeCloseContext(fc)

	if c := fc.logger.Check(zapcore.DebugLevel, "request handling finished"); c != nil {
		c.Write(zap.String("url", r.RequestURI))
	}
}
