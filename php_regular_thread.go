package frankenphp

import (
	"net/http"
	"sync"
	"strconv"

	"go.uber.org/zap"
)

type phpRegularThread struct {
	state  *threadStateHandler
	thread *phpThread
	worker *worker
	isDone   bool
	pinner   *runtime.Pinner
	getActiveRequest *http.Request
	knownVariableKeys map[string]*C.zend_string
}

// this is done once
func (thread *phpWorkerThread) onStartup(){
    // do nothing
}

func (thread *phpWorkerThread) pinner() *runtime.Pinner {
	return thread.pinner
}

func (thread *phpWorkerThread) getActiveRequest() *http.Request {
	return thread.activeRequest
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
    case stateReady, stateActive:
		return waitForScriptExecution(thread)
	}
}

// return true if the worker should continue to run
func (thread *phpWorkerThread) afterScriptExecution() bool {
	tearDownWorkerScript(thread, thread.worker)
	currentState := w.state.get()
	switch currentState {
	case stateDrain:
		thread.requestChan = make(chan *http.Request)
        return true
	case stateShuttingDown:
		return false
	}
	return true
}

func (thread *phpWorkerThread) onShutdown(){
    state.set(stateDone)
}

func waitForScriptExecution(thread *phpThread) string {
	select {
    case <-done:
        // no script should be executed if the server is shutting down
        thread.scriptName = ""
        return

    case r := <-requestChan:
        thread.mainRequest = r
        fc := r.Context().Value(contextKey).(*FrankenPHPContext)

        if err := updateServerContext(thread, r, true, false); err != nil {
            rejectRequest(fc.responseWriter, err.Error())
            afterRequest(thread, 0)
            thread.Unpin()
            // no script should be executed if the request was rejected
            return ""
        }

        // set the scriptName that should be executed
        return fc.scriptFilename
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