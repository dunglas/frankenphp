package frankenphp

import (
	"net/http"
	"sync"
	"strconv"

	"go.uber.org/zap"
)

type phpWorkerThread struct {
	state  *threadStateHandler
	thread *phpThread
	worker *worker
	isDone   bool
	pinner   *runtime.Pinner
	mainRequest *http.Request
	workerRequest *http.Request
	knownVariableKeys map[string]*C.zend_string
}

// this is done once
func (thread *phpWorkerThread) onStartup(){
    thread.requestChan = make(chan *http.Request)
    thread.backoff = newExponentialBackoff()
    thread.worker.threadMutex.Lock()
    thread.worker.threads = append(worker.threads, thread)
    thread.worker.threadMutex.Unlock()
}

func (thread *phpWorkerThread) pinner() *runtime.Pinner {
	return thread.pinner
}

func (thread *phpWorkerThread) getActiveRequest() *http.Request {
	if thread.workerRequest != nil {
		return thread.workerRequest
	}

	return thread.mainRequest
}

// return the name of the script or an empty string if no script should be executed
func (thread *phpWorkerThread) beforeScriptExecution() string {
	currentState := w.state.get()
	switch currentState {
	case stateInactive:
		thread.state.waitFor(stateActive, stateShuttingDown)
		return thread.beforeScriptExecution()
	case stateShuttingDown:
		return ""
	case stateRestarting:
		thread.state.waitFor(stateReady, stateShuttingDown)
		setUpWorkerScript(thread, thread.worker)
        return thread.worker.fileName
    case stateReady, stateActive:
		setUpWorkerScript(w.thread, w.worker)
		return thread.worker.fileName
	}
}

func (thread *phpWorkerThread) waitForWorkerRequest() bool {

    if c := logger.Check(zapcore.DebugLevel, "waiting for request"); c != nil {
        c.Write(zap.String("worker", thread.worker.fileName))
    }

	if !thread.state.is(stateReady) {
		metrics.ReadyWorker(w.worker.fileName)
		thread.state.set(stateReady)
	}

    var r *http.Request
    select {
    case <-workersDone:
        if c := logger.Check(zapcore.DebugLevel, "shutting down"); c != nil {
            c.Write(zap.String("worker", thread.worker.fileName))
        }

        // execute opcache_reset if the restart was triggered by the watcher
        if watcherIsEnabled && thread.state.is(stateRestarting) {
            C.frankenphp_reset_opcache()
        }

        return false
    case r = <-thread.requestChan:
    case r = <-thread.worker.requestChan:
    }

    thread.workerRequest = r

    if c := logger.Check(zapcore.DebugLevel, "request handling started"); c != nil {
        c.Write(zap.String("worker", thread.worker.fileName), zap.String("url", r.RequestURI))
    }

    if err := updateServerContext(thread, r, false, true); err != nil {
        // Unexpected error
        if c := logger.Check(zapcore.DebugLevel, "unexpected error"); c != nil {
            c.Write(zap.String("worker", thread.worker.fileName), zap.String("url", r.RequestURI), zap.Error(err))
        }
        fc := r.Context().Value(contextKey).(*FrankenPHPContext)
        rejectRequest(fc.responseWriter, err.Error())
        maybeCloseContext(fc)
        thread.workerRequest = nil
        thread.Unpin()

        return go_frankenphp_worker_handle_request_start(threadIndex)
    }
    return true
}

// return true if the worker should continue to run
func (thread *phpWorkerThread) afterScriptExecution() bool {
	tearDownWorkerScript(thread, thread.worker)
	currentState := w.state.get()
	switch currentState {
	case stateDrain:
		thread.requestChan = make(chan *http.Request)
        return true
    }
	case stateShuttingDown:
		return false
	}
	return true
}

func (thread *phpWorkerThread) onShutdown(){
    state.set(stateDone)
}

func setUpWorkerScript(thread *phpThread, worker *worker) {
	thread.worker = worker
	// if we are restarting due to file watching, set the state back to ready
	if thread.state.is(stateRestarting) {
		thread.state.set(stateReady)
	}

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

	if err := updateServerContext(thread, r, true, false); err != nil {
		panic(err)
	}

	thread.mainRequest = r
	if c := logger.Check(zapcore.DebugLevel, "starting"); c != nil {
		c.Write(zap.String("worker", worker.fileName), zap.Int("thread", thread.threadIndex))
	}
}

func tearDownWorkerScript(thread *phpThread, exitStatus int) {
	fc := thread.mainRequest.Context().Value(contextKey).(*FrankenPHPContext)
	fc.exitStatus = exitStatus

	defer func() {
		maybeCloseContext(fc)
		thread.mainRequest = nil
	}()

	// on exit status 0 we just run the worker script again
	worker := thread.worker
	if fc.exitStatus == 0 {
		// TODO: make the max restart configurable
		metrics.StopWorker(worker.fileName, StopReasonRestart)

		if c := logger.Check(zapcore.DebugLevel, "restarting"); c != nil {
			c.Write(zap.String("worker", worker.fileName))
		}
		return
	}

	// on exit status 1 we apply an exponential backoff when restarting
	metrics.StopWorker(worker.fileName, StopReasonCrash)
	thread.backoff.trigger(func(failureCount int) {
		// if we end up here, the worker has not been up for backoff*2
		// this is probably due to a syntax error or another fatal error
		if !watcherIsEnabled {
			panic(fmt.Errorf("workers %q: too many consecutive failures", worker.fileName))
		}
		logger.Warn("many consecutive worker failures", zap.String("worker", worker.fileName), zap.Int("failures", failureCount))
	})
}

func (thread *phpWorkerThread) getKnownVariableKeys() map[string]*C.zend_string{
	return thread.knownVariableKeys
}
func (thread *phpWorkerThread) setKnownVariableKeys(map[string]*C.zend_string){
	thread.knownVariableKeys = knownVariableKeys
}