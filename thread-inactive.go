package frankenphp

import (
	"net/http"
	"time"
)

// representation of a thread with no work assigned to it
// implements the threadHandler interface
// each inactive thread weighs around ~350KB
// keeping threads at 'inactive' will consume more memory, but allow a faster transition
type inactiveThread struct {
	thread *phpThread
}

func convertToInactiveThread(thread *phpThread) {
	thread.setHandler(&inactiveThread{thread: thread})
}

func (handler *inactiveThread) beforeScriptExecution() string {
	thread := handler.thread

	switch thread.state.get() {
	case stateTransitionRequested:
		return thread.transitionToNewHandler()
	case stateBooting, stateTransitionComplete:
		thread.state.set(stateInactive)
		thread.waitingSince = time.Now().UnixMilli()
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
