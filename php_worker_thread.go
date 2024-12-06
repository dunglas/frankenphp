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

type phpWorkerThread struct {
	state  *stateHandler
	thread *phpThread
	worker *worker
	mainRequest *http.Request
	workerRequest *http.Request
	backoff *exponentialBackoff
}

func convertToWorkerThread(thread *phpThread, worker *worker) {
	thread.handler = &phpWorkerThread{
		state: thread.state,
		thread: thread,
		worker: worker,
	}
	thread.handler.onStartup()
	thread.state.set(stateActive)
}

// this is done once
func (handler *phpWorkerThread) onStartup(){
    handler.thread.requestChan = make(chan *http.Request)
    handler.backoff = newExponentialBackoff()
    handler.worker.threadMutex.Lock()
    handler.worker.threads = append(handler.worker.threads, handler.thread)
    handler.worker.threadMutex.Unlock()
}

func (handler *phpWorkerThread) getActiveRequest() *http.Request {
	if handler.workerRequest != nil {
		return handler.workerRequest
	}

	return handler.mainRequest
}

func (t *phpWorkerThread) isReadyToTransition() bool {
	return false
}

// return the name of the script or an empty string if no script should be executed
func (handler *phpWorkerThread) beforeScriptExecution() string {
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

func (handler *phpWorkerThread) waitForWorkerRequest() bool {

    if c := logger.Check(zapcore.DebugLevel, "waiting for request"); c != nil {
        c.Write(zap.String("worker", handler.worker.fileName))
    }

	if !handler.state.is(stateReady) {
		metrics.ReadyWorker(handler.worker.fileName)
		handler.state.set(stateReady)
	}

    var r *http.Request
    select {
    case <-workersDone:
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
func (handler *phpWorkerThread) afterScriptExecution(exitStatus int) bool {
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

func (handler *phpWorkerThread) onShutdown(){
    handler.state.set(stateDone)
}

func setUpWorkerScript(handler *phpWorkerThread, worker *worker) {
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

	handler.mainRequest = r
	if c := logger.Check(zapcore.DebugLevel, "starting"); c != nil {
		c.Write(zap.String("worker", worker.fileName), zap.Int("thread", handler.thread.threadIndex))
	}
}

func tearDownWorkerScript(handler *phpWorkerThread, exitStatus int) {
	fc := handler.mainRequest.Context().Value(contextKey).(*FrankenPHPContext)
	fc.exitStatus = exitStatus

	defer func() {
		maybeCloseContext(fc)
		handler.mainRequest = nil
		handler.workerRequest = nil
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
	handler := phpThreads[threadIndex].handler.(*phpWorkerThread)
	return C.bool(handler.waitForWorkerRequest())
}

//export go_frankenphp_finish_worker_request
func go_frankenphp_finish_worker_request(threadIndex C.uintptr_t) {
	thread := phpThreads[threadIndex]
	r := thread.getActiveRequest()
	fc := r.Context().Value(contextKey).(*FrankenPHPContext)

	maybeCloseContext(fc)
	thread.handler.(*phpWorkerThread).workerRequest = nil
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