package frankenphp

import (
	"net/http"
	"strconv"
)

// representation of a thread with no work assigned to it
// implements the threadHandler interface
type inactiveThread struct {
	state  *threadState
}

func convertToInactiveThread(thread *phpThread) {
	thread.handler = &inactiveThread{state: thread.state}
}

func (t *inactiveThread) isReadyToTransition() bool {
	return true
}

func (thread *inactiveThread) getActiveRequest() *http.Request {
	panic("idle threads have no requests")
}

func (thread *inactiveThread) beforeScriptExecution() string {
	// no script execution for inactive threads
	return ""
}

func (thread *inactiveThread) afterScriptExecution(exitStatus int) bool {
	thread.state.set(stateInactive)
	// wait for external signal to start or shut down
	thread.state.waitFor(stateActive, stateShuttingDown)
	switch thread.state.get() {
	case stateActive:
		return true
	case stateShuttingDown:
		return false
	}
	panic("unexpected state: "+strconv.Itoa(int(thread.state.get())))
}

func (thread *inactiveThread) onShutdown(){
    thread.state.set(stateDone)
}

