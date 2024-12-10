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
	thread.setHandler(&regularThread{
		thread: thread,
		state:  thread.state,
	})
}

// beforeScriptExecution returns the name of the script or an empty string on shutdown
func (handler *regularThread) beforeScriptExecution() string {
	switch handler.state.get() {
	case stateTransitionRequested:
		return handler.thread.transitionToNewHandler()
	case stateTransitionComplete:
		handler.state.set(stateReady)
		return handler.waitForRequest()
	case stateReady:
		return handler.waitForRequest()
	case stateShuttingDown:
		// signal to stop
		return ""
	}
	panic("unexpected state: " + handler.state.name())
}

// return true if the worker should continue to run
func (handler *regularThread) afterScriptExecution(exitStatus int) {
	handler.afterRequest(exitStatus)
}

func (handler *regularThread) getActiveRequest() *http.Request {
	return handler.activeRequest
}

func (handler *regularThread) waitForRequest() string {
	select {
	case <-handler.thread.drainChan:
		// go back to beforeScriptExecution
		return handler.beforeScriptExecution()

	case r := <-requestChan:
		handler.activeRequest = r
		fc := r.Context().Value(contextKey).(*FrankenPHPContext)

		if err := updateServerContext(handler.thread, r, true, false); err != nil {
			rejectRequest(fc.responseWriter, err.Error())
			handler.afterRequest(0)
			// go back to beforeScriptExecution
			return handler.beforeScriptExecution()
		}

		// set the scriptFilename that should be executed
		return fc.scriptFilename
	}
}

func (handler *regularThread) afterRequest(exitStatus int) {
	fc := handler.activeRequest.Context().Value(contextKey).(*FrankenPHPContext)
	fc.exitStatus = exitStatus
	maybeCloseContext(fc)
	handler.activeRequest = nil
}
