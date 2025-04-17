package frankenphp

// #include "frankenphp.h"
import "C"
import (
	"path/filepath"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// representation of a thread assigned to a worker script
// executes the PHP worker script in a loop
// implements the threadHandler interface
type workerThread struct {
	state           *threadState
	thread          *phpThread
	worker          *worker
	dummyContext    *frankenPHPContext
	workerContext   *frankenPHPContext
	backoff         *exponentialBackoff
	isBootingScript bool // true if the worker has not reached frankenphp_handle_request yet
}

func convertToWorkerThread(thread *phpThread, worker *worker) {
	thread.setHandler(&workerThread{
		state:  thread.state,
		thread: thread,
		worker: worker,
		backoff: &exponentialBackoff{
			maxBackoff:             1 * time.Second,
			minBackoff:             100 * time.Millisecond,
			maxConsecutiveFailures: 6,
		},
	})
	worker.attachThread(thread)
}

// beforeScriptExecution returns the name of the script or an empty string on shutdown
func (handler *workerThread) beforeScriptExecution() string {
	switch handler.state.get() {
	case stateTransitionRequested:
		handler.worker.detachThread(handler.thread)
		return handler.thread.transitionToNewHandler()
	case stateRestarting:
		handler.state.set(stateYielding)
		handler.state.waitFor(stateReady, stateShuttingDown)
		return handler.beforeScriptExecution()
	case stateReady, stateTransitionComplete:
		setupWorkerScript(handler, handler.worker)
		return handler.worker.fileName
	case stateShuttingDown:
		handler.worker.detachThread(handler.thread)
		// signal to stop
		return ""
	}
	panic("unexpected state: " + handler.state.name())
}

func (handler *workerThread) afterScriptExecution(exitStatus int) {
	tearDownWorkerScript(handler, exitStatus)
}

func (handler *workerThread) getRequestContext() *frankenPHPContext {
	if handler.workerContext != nil {
		return handler.workerContext
	}

	return handler.dummyContext
}

func (handler *workerThread) name() string {
	return "Worker PHP Thread - " + handler.worker.fileName
}

func setupWorkerScript(handler *workerThread, worker *worker) {
	handler.backoff.wait()
	metrics.StartWorker(worker.name)

	if handler.state.is(stateReady) {
		metrics.ReadyWorker(handler.worker.name)
	}

	// Create a dummy request to set up the worker
	fc, err := newDummyContext(
		filepath.Base(worker.fileName),
		WithRequestDocumentRoot(filepath.Dir(worker.fileName), false),
		WithRequestPreparedEnv(worker.env),
	)
	if err != nil {
		panic(err)
	}

	if err := updateServerContext(handler.thread, fc, false); err != nil {
		panic(err)
	}

	handler.dummyContext = fc
	handler.isBootingScript = true
	clearSandboxedEnv(handler.thread)
	if c := logger.Check(zapcore.DebugLevel, "starting"); c != nil {
		c.Write(zap.String("worker", worker.name), zap.Int("thread", handler.thread.threadIndex))
	}
}

func tearDownWorkerScript(handler *workerThread, exitStatus int) {
	worker := handler.worker
	handler.dummyContext = nil

	// if the worker request is not nil, the script might have crashed
	// make sure to close the worker request context
	if handler.workerContext != nil {
		handler.workerContext.closeContext()
		handler.workerContext = nil
	}

	// on exit status 0 we just run the worker script again
	if exitStatus == 0 && !handler.isBootingScript {
		// TODO: make the max restart configurable
		metrics.StopWorker(worker.name, StopReasonRestart)
		handler.backoff.recordSuccess()
		if c := logger.Check(zapcore.DebugLevel, "restarting"); c != nil {
			c.Write(zap.String("worker", worker.name))
		}
		return
	}

	// worker has thrown a fatal error or has not reached frankenphp_handle_request
	metrics.StopWorker(worker.name, StopReasonCrash)

	if !handler.isBootingScript {
		// fatal error (could be due to timeouts, etc.)
		return
	}

	logger.Error("worker script has not reached frankenphp_handle_request", zap.String("worker", worker.name))

	// panic after exponential backoff if the worker has never reached frankenphp_handle_request
	if handler.backoff.recordFailure() {
		if !watcherIsEnabled && !handler.state.is(stateReady) {
			logger.Panic("too many consecutive worker failures", zap.String("worker", worker.name), zap.Int("failures", handler.backoff.failureCount))
		}
		logger.Warn("many consecutive worker failures", zap.String("worker", worker.name), zap.Int("failures", handler.backoff.failureCount))
	}
}

// waitForWorkerRequest is called during frankenphp_handle_request in the php worker script.
func (handler *workerThread) waitForWorkerRequest() bool {
	// unpin any memory left over from previous requests
	handler.thread.Unpin()

	if c := logger.Check(zapcore.DebugLevel, "waiting for request"); c != nil {
		c.Write(zap.String("worker", handler.worker.name))
	}

	// Clear the first dummy request created to initialize the worker
	if handler.isBootingScript {
		handler.isBootingScript = false
		if !C.frankenphp_shutdown_dummy_request() {
			panic("Not in CGI context")
		}
	}

	// worker threads are 'ready' after they first reach frankenphp_handle_request()
	// 'stateTransitionComplete' is only true on the first boot of the worker script,
	// while 'isBootingScript' is true on every boot of the worker script
	if handler.state.is(stateTransitionComplete) {
		metrics.ReadyWorker(handler.worker.name)
		handler.state.set(stateReady)
	}

	handler.state.markAsWaiting(true)

	var fc *frankenPHPContext
	select {
	case <-handler.thread.drainChan:
		if c := logger.Check(zapcore.DebugLevel, "shutting down"); c != nil {
			c.Write(zap.String("worker", handler.worker.name))
		}

		// flush the opcache when restarting due to watcher or admin api
		// note: this is done right before frankenphp_handle_request() returns 'false'
		if handler.state.is(stateRestarting) {
			C.frankenphp_reset_opcache()
		}

		return false
	case fc = <-handler.thread.requestChan:
	case fc = <-handler.worker.requestChan:
	}

	handler.workerContext = fc
	handler.state.markAsWaiting(false)

	if c := logger.Check(zapcore.DebugLevel, "request handling started"); c != nil {
		c.Write(zap.String("worker", handler.worker.name), zap.String("url", fc.request.RequestURI))
	}

	if err := updateServerContext(handler.thread, fc, true); err != nil {
		// Unexpected error or invalid request
		if c := logger.Check(zapcore.DebugLevel, "unexpected error"); c != nil {
			c.Write(zap.String("worker", handler.worker.name), zap.String("url", fc.request.RequestURI), zap.Error(err))
		}
		fc.rejectBadRequest(err.Error())
		handler.workerContext = nil

		return handler.waitForWorkerRequest()
	}

	return true
}

// go_frankenphp_worker_handle_request_start is called at the start of every php request served.
//
//export go_frankenphp_worker_handle_request_start
func go_frankenphp_worker_handle_request_start(threadIndex C.uintptr_t) C.bool {
	handler := phpThreads[threadIndex].handler.(*workerThread)
	return C.bool(handler.waitForWorkerRequest())
}

// go_frankenphp_finish_worker_request is called at the end of every php request served.
//
//export go_frankenphp_finish_worker_request
func go_frankenphp_finish_worker_request(threadIndex C.uintptr_t) {
	thread := phpThreads[threadIndex]
	fc := thread.getRequestContext()

	fc.closeContext()
	thread.handler.(*workerThread).workerContext = nil

	if c := fc.logger.Check(zapcore.DebugLevel, "request handling finished"); c != nil {
		c.Write(zap.String("worker", fc.scriptFilename), zap.String("url", fc.request.RequestURI))
	}
}

// when frankenphp_finish_request() is directly called from PHP
//
//export go_frankenphp_finish_php_request
func go_frankenphp_finish_php_request(threadIndex C.uintptr_t) {
	fc := phpThreads[threadIndex].getRequestContext()
	fc.closeContext()

	if c := fc.logger.Check(zapcore.DebugLevel, "request handling finished"); c != nil {
		c.Write(zap.String("url", fc.request.RequestURI))
	}
}
