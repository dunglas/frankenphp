package frankenphp

// #include "frankenphp.h"
import "C"
import (
	"net/http"
	"path/filepath"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// representation of a thread assigned to a worker script
// executes the PHP worker script in a loop
// implements the threadHandler interface
type workerThread struct {
	state         *threadState
	thread        *phpThread
	worker        *worker
	fakeRequest   *http.Request
	workerRequest *http.Request
	backoff       *exponentialBackoff
	inRequest     bool // true if the worker is currently handling a request
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
		return handler.restartGracefully()
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

func (handler *workerThread) getActiveRequest() *http.Request {
	if handler.workerRequest != nil {
		return handler.workerRequest
	}

	return handler.fakeRequest
}

func (handler *workerThread) name() string {
	return "Worker PHP Thread - " + handler.worker.fileName
}

func setupWorkerScript(handler *workerThread, worker *worker) {
	handler.backoff.wait()
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

	if err := updateServerContext(handler.thread, r, true, false); err != nil {
		panic(err)
	}

	handler.setFakeRequest(r)
	if c := logger.Check(zapcore.DebugLevel, "starting"); c != nil {
		c.Write(zap.String("worker", worker.fileName), zap.Int("thread", handler.thread.threadIndex))
	}
}

func tearDownWorkerScript(handler *workerThread, exitStatus int) {
	// if the worker request is not nil, the script might have crashed
	// make sure to close the worker request context
	if handler.workerRequest != nil {
		fc := handler.workerRequest.Context().Value(contextKey).(*FrankenPHPContext)
		maybeCloseContext(fc)
		handler.setWorkerRequest(nil)
	}

	fc := handler.fakeRequest.Context().Value(contextKey).(*FrankenPHPContext)
	fc.exitStatus = exitStatus
	handler.setFakeRequest(nil)

	// on exit status 0 we just run the worker script again
	worker := handler.worker
	if fc.exitStatus == 0 {
		// TODO: make the max restart configurable
		metrics.StopWorker(worker.fileName, StopReasonRestart)
		handler.backoff.recordSuccess()
		if c := logger.Check(zapcore.DebugLevel, "restarting"); c != nil {
			c.Write(zap.String("worker", worker.fileName))
		}
		return
	}

	// TODO: error status

	// on exit status 1 we apply an exponential backoff when restarting
	metrics.StopWorker(worker.fileName, StopReasonCrash)
	if !handler.inRequest && handler.backoff.recordFailure() {
		if !watcherIsEnabled {
			logger.Panic("too many consecutive worker failures", zap.String("worker", worker.fileName), zap.Int("failures", handler.backoff.failureCount))
		}
		logger.Warn("many consecutive worker failures", zap.String("worker", worker.fileName), zap.Int("failures", handler.backoff.failureCount))
	}
}

// waitForWorkerRequest is called during frankenphp_handle_request in the php worker script.
func (handler *workerThread) waitForWorkerRequest() bool {
	// unpin any memory left over from previous requests
	handler.thread.Unpin()
	handler.state.markAsWaiting(true)

	if c := logger.Check(zapcore.DebugLevel, "waiting for request"); c != nil {
		c.Write(zap.String("worker", handler.worker.fileName))
	}

	if handler.state.compareAndSwap(stateTransitionComplete, stateReady) {
		metrics.ReadyWorker(handler.worker.fileName)
	}

	var r *http.Request
	select {
	case <-handler.thread.drainChan:
		if c := logger.Check(zapcore.DebugLevel, "shutting down"); c != nil {
			c.Write(zap.String("worker", handler.worker.fileName))
		}

		return false
	case r = <-handler.thread.requestChan:
	case r = <-handler.worker.requestChan:
	}

	handler.setWorkerRequest(r)
	handler.state.markAsWaiting(false)

	if c := logger.Check(zapcore.DebugLevel, "request handling started"); c != nil {
		c.Write(zap.String("worker", handler.worker.fileName), zap.String("url", r.RequestURI))
	}
	handler.inRequest = true

	if err := updateServerContext(handler.thread, r, false, true); err != nil {
		// Unexpected error or invalid request
		if c := logger.Check(zapcore.DebugLevel, "unexpected error"); c != nil {
			c.Write(zap.String("worker", handler.worker.fileName), zap.String("url", r.RequestURI), zap.Error(err))
		}
		fc := r.Context().Value(contextKey).(*FrankenPHPContext)
		rejectRequest(fc.responseWriter, err.Error())
		maybeCloseContext(fc)
		handler.workerRequest = nil

		return handler.waitForWorkerRequest()
	}

	return true
}

func (handler *workerThread) setWorkerRequest(r *http.Request) {
	handler.thread.requestMu.Lock()
	handler.workerRequest = r
	handler.thread.requestMu.Unlock()
}

func (handler *workerThread) setFakeRequest(r *http.Request) {
	handler.thread.requestMu.Lock()
	handler.fakeRequest = r
	handler.thread.requestMu.Unlock()
}

// When restarting gracefully, all threads wait for each other to finish
// opcache_reset will be called once all threads are yielding
func (handler *workerThread) restartGracefully() string {
	handler.state.set(stateYielding)
	handler.state.waitFor(stateReady, stateShuttingDown, stateOpcacheReset)

	// one thread will be marked to flush the opcache
	// this will avoid a race condition in opcache under high concurrency
	if handler.state.is(stateOpcacheReset) {
		C.frankenphp_reset_opcache()
		logger.Debug("opcache reset", zap.Int("threadIndex", handler.thread.threadIndex))
		handler.state.set(stateYielding)
		handler.state.waitFor(stateReady, stateShuttingDown)
	}

	return handler.beforeScriptExecution()
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
	r := thread.getActiveRequest()
	fc := r.Context().Value(contextKey).(*FrankenPHPContext)

	maybeCloseContext(fc)
	thread.handler.(*workerThread).workerRequest = nil

	if c := fc.logger.Check(zapcore.DebugLevel, "request handling finished"); c != nil {
		c.Write(zap.String("worker", fc.scriptFilename), zap.String("url", r.RequestURI))
	}
}

// when frankenphp_finish_request() is directly called from PHP
//
//export go_frankenphp_finish_php_request
func go_frankenphp_finish_php_request(threadIndex C.uintptr_t) {
	r := phpThreads[threadIndex].getActiveRequest()
	fc := r.Context().Value(contextKey).(*FrankenPHPContext)
	maybeCloseContext(fc)

	if c := fc.logger.Check(zapcore.DebugLevel, "request handling finished"); c != nil {
		c.Write(zap.String("url", r.RequestURI))
	}
}
