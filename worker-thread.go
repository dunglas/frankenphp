package frankenphp

// #include "frankenphp.h"
import "C"
import (
	"net/http"
	"path/filepath"
	"fmt"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// representation of a thread assigned to a worker script
// executes the PHP worker script in a loop
// implements the threadHandler interface
type workerThread struct {
	state  *threadState
	thread *phpThread
	worker *worker
	fakeRequest *http.Request
	workerRequest *http.Request
	backoff *exponentialBackoff
}

func convertToWorkerThread(thread *phpThread, worker *worker) {
	handler := &workerThread{
		state: thread.state,
		thread: thread,
		worker: worker,
		backoff: newExponentialBackoff(),
	}
	thread.handler = handler
	thread.requestChan = make(chan *http.Request)
	worker.threadMutex.Lock()
    worker.threads = append(worker.threads, thread)
    worker.threadMutex.Unlock()

	thread.state.set(stateActive)
}

func (handler *workerThread) getActiveRequest() *http.Request {
	if handler.workerRequest != nil {
		return handler.workerRequest
	}

	return handler.fakeRequest
}

func (t *workerThread) isReadyToTransition() bool {
	return false
}

// return the name of the script or an empty string if no script should be executed
func (handler *workerThread) beforeScriptExecution() string {
	currentState := handler.state.get()
	switch currentState {
	case stateInactive:
		handler.state.waitFor(stateActive, stateShuttingDown)
		return handler.beforeScriptExecution()
	case stateShuttingDown:
		return ""
	case stateRestarting:
		handler.state.set(stateYielding)
		handler.state.waitFor(stateReady, stateShuttingDown)
        return handler.beforeScriptExecution()
    case stateReady, stateActive:
		setUpWorkerScript(handler, handler.worker)
		return handler.worker.fileName
	}
    // TODO: panic?
	return ""
}

func (handler *workerThread) waitForWorkerRequest() bool {

    if c := logger.Check(zapcore.DebugLevel, "waiting for request"); c != nil {
        c.Write(zap.String("worker", handler.worker.fileName))
    }

	if handler.state.is(stateActive) {
		metrics.ReadyWorker(handler.worker.fileName)
		handler.state.set(stateReady)
	}

    var r *http.Request
    select {
    case <-handler.thread.drainChan:
        if c := logger.Check(zapcore.DebugLevel, "shutting down"); c != nil {
            c.Write(zap.String("worker", handler.worker.fileName))
        }

        // execute opcache_reset if the restart was triggered by the watcher
        if watcherIsEnabled && handler.state.is(stateRestarting) {
            C.frankenphp_reset_opcache()
        }

        return false
    case r = <-handler.thread.requestChan:
    case r = <-handler.worker.requestChan:
    }

    handler.workerRequest = r

    if c := logger.Check(zapcore.DebugLevel, "request handling started"); c != nil {
        c.Write(zap.String("worker", handler.worker.fileName), zap.String("url", r.RequestURI))
    }

    if err := updateServerContext(handler.thread, r, false, true); err != nil {
        // Unexpected error
        if c := logger.Check(zapcore.DebugLevel, "unexpected error"); c != nil {
            c.Write(zap.String("worker", handler.worker.fileName), zap.String("url", r.RequestURI), zap.Error(err))
        }
        fc := r.Context().Value(contextKey).(*FrankenPHPContext)
        rejectRequest(fc.responseWriter, err.Error())
        maybeCloseContext(fc)
        handler.workerRequest = nil
        handler.thread.Unpin()

        return handler.waitForWorkerRequest()
    }
    return true
}

// return true if the worker should continue to run
func (handler *workerThread) afterScriptExecution(exitStatus int) bool {
	tearDownWorkerScript(handler, exitStatus)
	currentState := handler.state.get()
	switch currentState {
	case stateDrain:
		handler.thread.requestChan = make(chan *http.Request)
        return true
	case stateShuttingDown:
		return false
	}
	return true
}

func (handler *workerThread) onShutdown(){
    handler.state.set(stateDone)
}

func setUpWorkerScript(handler *workerThread, worker *worker) {
	handler.backoff.reset()
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

	handler.fakeRequest = r
	if c := logger.Check(zapcore.DebugLevel, "starting"); c != nil {
		c.Write(zap.String("worker", worker.fileName), zap.Int("thread", handler.thread.threadIndex))
	}
}

func tearDownWorkerScript(handler *workerThread, exitStatus int) {

	// if the fake request is nil, no script was executed
	if handler.fakeRequest == nil {
		return
	}

	// if the worker request is not nil, the script might have crashed
	// make sure to close the worker request context
	if handler.workerRequest != nil {
		fc := handler.workerRequest.Context().Value(contextKey).(*FrankenPHPContext)
		maybeCloseContext(fc)
		handler.workerRequest = nil
	}

	fc := handler.fakeRequest.Context().Value(contextKey).(*FrankenPHPContext)
	fc.exitStatus = exitStatus

	defer func() {
		handler.fakeRequest = nil
	}()

	// on exit status 0 we just run the worker script again
	worker := handler.worker
	if fc.exitStatus == 0 {
		// TODO: make the max restart configurable
		metrics.StopWorker(worker.fileName, StopReasonRestart)

		if c := logger.Check(zapcore.DebugLevel, "restarting"); c != nil {
			c.Write(zap.String("worker", worker.fileName))
		}
		return
	}

	// TODO: error status

	// on exit status 1 we apply an exponential backoff when restarting
	metrics.StopWorker(worker.fileName, StopReasonCrash)
	handler.backoff.trigger(func(failureCount int) {
		// if we end up here, the worker has not been up for backoff*2
		// this is probably due to a syntax error or another fatal error
		if !watcherIsEnabled {
			panic(fmt.Errorf("workers %q: too many consecutive failures", worker.fileName))
		}
		logger.Warn("many consecutive worker failures", zap.String("worker", worker.fileName), zap.Int("failures", failureCount))
	})
}

//export go_frankenphp_worker_handle_request_start
func go_frankenphp_worker_handle_request_start(threadIndex C.uintptr_t) C.bool {
	handler := phpThreads[threadIndex].handler.(*workerThread)
	return C.bool(handler.waitForWorkerRequest())
}

//export go_frankenphp_finish_worker_request
func go_frankenphp_finish_worker_request(threadIndex C.uintptr_t) {
	thread := phpThreads[threadIndex]
	r := thread.getActiveRequest()
	fc := r.Context().Value(contextKey).(*FrankenPHPContext)

	maybeCloseContext(fc)
	thread.handler.(*workerThread).workerRequest = nil
	thread.Unpin()

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