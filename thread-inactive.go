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
	thread.handler = &inactiveThread{thread: thread}
}

func (thread *inactiveThread) getActiveRequest() *http.Request {
	panic("inactive threads have no requests")
}

func (handler *inactiveThread) beforeScriptExecution() string {
	thread := handler.thread
	thread.state.set(stateInactive)

	// wait for external signal to start or shut down
	thread.state.waitFor(stateTransitionRequested, stateShuttingDown)
	switch thread.state.get() {
	case stateTransitionRequested:
		thread.state.set(stateTransitionInProgress)
		thread.state.waitFor(stateTransitionComplete, stateShuttingDown)
		// execute beforeScriptExecution of the new handler
		return thread.handler.beforeScriptExecution()
	case stateShuttingDown:
		// signal to stop
		return ""
	}
	panic("unexpected state: " + thread.state.name())
}

func (thread *inactiveThread) afterScriptExecution(exitStatus int) {
	panic("inactive threads should not execute scripts")
}
