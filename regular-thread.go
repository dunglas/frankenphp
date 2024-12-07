package frankenphp

// #include "frankenphp.h"
import "C"
import (
	"net/http"
)

// representation of a non-worker PHP thread
// executes PHP scripts in a web context
// implements the threadHandler interface
type regularThread struct {
	state         *threadState
	thread        *phpThread
	activeRequest *http.Request
}

func convertToRegularThread(thread *phpThread) {
	thread.handler = &regularThread{
		thread: thread,
		state:  thread.state,
	}
	thread.state.set(stateActive)
}

func (t *regularThread) isReadyToTransition() bool {
	return false
}

func (handler *regularThread) getActiveRequest() *http.Request {
	return handler.activeRequest
}

// return the name of the script or an empty string if no script should be executed
func (handler *regularThread) beforeScriptExecution() string {
	currentState := handler.state.get()
	switch currentState {
	case stateInactive:
		handler.state.waitFor(stateActive, stateShuttingDown)
		return handler.beforeScriptExecution()
	case stateShuttingDown:
		return ""
	case stateReady, stateActive:
		return handler.waitForScriptExecution()
	}
	return ""
}

// return true if the worker should continue to run
func (handler *regularThread) afterScriptExecution(exitStatus int) bool {
	handler.afterRequest(exitStatus)

	currentState := handler.state.get()
	switch currentState {
	case stateDrain:
		return true
	case stateShuttingDown:
		return false
	}
	return true
}

func (handler *regularThread) onShutdown() {
	handler.state.set(stateDone)
}

func (handler *regularThread) waitForScriptExecution() string {
	select {
	case <-handler.thread.drainChan:
		// no script should be executed if the server is shutting down
		return ""

	case r := <-requestChan:
		handler.activeRequest = r
		fc := r.Context().Value(contextKey).(*FrankenPHPContext)

		if err := updateServerContext(handler.thread, r, true, false); err != nil {
			rejectRequest(fc.responseWriter, err.Error())
			handler.afterRequest(0)
			handler.thread.Unpin()
			// no script should be executed if the request was rejected
			return ""
		}

		// set the scriptName that should be executed
		return fc.scriptFilename
	}
}

func (handler *regularThread) afterRequest(exitStatus int) {

	// if the request is nil, no script was executed
	if handler.activeRequest == nil {
		return
	}

	fc := handler.activeRequest.Context().Value(contextKey).(*FrankenPHPContext)
	fc.exitStatus = exitStatus
	maybeCloseContext(fc)
	handler.activeRequest = nil
}
