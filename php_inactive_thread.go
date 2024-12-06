package frankenphp

import (
	"net/http"
	"strconv"
)

type phpInactiveThread struct {
	state  *stateHandler
}

func convertToInactiveThread(thread *phpThread) {
	thread.handler = &phpInactiveThread{state: thread.state}
}

func (t *phpInactiveThread) isReadyToTransition() bool {
	return true
}

// this is done once
func (thread *phpInactiveThread) onStartup(){
    // do nothing
}

func (thread *phpInactiveThread) getActiveRequest() *http.Request {
	panic("idle threads have no requests")
}

func (thread *phpInactiveThread) beforeScriptExecution() string {
	thread.state.set(stateInactive)
	return ""
}

func (thread *phpInactiveThread) afterScriptExecution(exitStatus int) bool {
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

func (thread *phpInactiveThread) onShutdown(){
    thread.state.set(stateDone)
}

