package frankenphp

import (
	"net/http"
)

// representation of a thread with no work assigned to it
// implements the threadHandler interface
type inactiveThread struct {
	thread *phpThread
}

func convertToInactiveThread(thread *phpThread) {
	if thread.handler == nil {
		thread.handler = &inactiveThread{thread: thread}
		return
	}
	thread.setHandler(&inactiveThread{thread: thread})
}

func (handler *inactiveThread) beforeScriptExecution() string {
	thread := handler.thread

	switch thread.state.get() {
	case stateTransitionRequested:
		return thread.transitionToNewHandler()
	case stateBooting, stateTransitionComplete:
		thread.state.set(stateInactive)
		// wait for external signal to start or shut down
		thread.state.waitFor(stateTransitionRequested, stateShuttingDown)
		return handler.beforeScriptExecution()
	case stateShuttingDown:
		// signal to stop
		return ""
	}
	panic("unexpected state: " + thread.state.name())
}

func (thread *inactiveThread) afterScriptExecution(exitStatus int) {
	panic("inactive threads should not execute scripts")
}

func (thread *inactiveThread) getActiveRequest() *http.Request {
	panic("inactive threads have no requests")
}
