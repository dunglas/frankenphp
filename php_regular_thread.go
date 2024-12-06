package frankenphp

// #include "frankenphp.h"
import "C"
import (
	"net/http"
	"go.uber.org/zap"
)

type phpRegularThread struct {
	state  *stateHandler
	thread *phpThread
	activeRequest *http.Request
}

func convertToRegularThread(thread *phpThread) {
	thread.handler = &phpRegularThread{
		thread: thread,
		state: thread.state,
	}
	thread.handler.onStartup()
	thread.state.set(stateActive)
}

func (t *phpRegularThread) isReadyToTransition() bool {
	return false
}

// this is done once
func (thread *phpRegularThread) onStartup(){
    // do nothing
}

func (thread *phpRegularThread) getActiveRequest() *http.Request {
	return thread.activeRequest
}

// return the name of the script or an empty string if no script should be executed
func (thread *phpRegularThread) beforeScriptExecution() string {
	currentState := thread.state.get()
	switch currentState {
	case stateInactive:
		logger.Info("waiting for activation", zap.Int("threadIndex", thread.thread.threadIndex),zap.Int("state", int(thread.state.get())))
		thread.state.waitFor(stateActive, stateShuttingDown)
		logger.Info("activated", zap.Int("threadIndex", thread.thread.threadIndex),zap.Int("state", int(thread.state.get())))
		return thread.beforeScriptExecution()
	case stateShuttingDown:
		return ""
    case stateReady, stateActive:
		logger.Info("beforeScriptExecution", zap.Int("state", int(thread.state.get())))
		return waitForScriptExecution(thread)
	}
	return ""
}

// return true if the worker should continue to run
func (thread *phpRegularThread) afterScriptExecution(exitStatus int) bool {
	thread.afterRequest(exitStatus)

	currentState := thread.state.get()
	switch currentState {
	case stateDrain:
        return true
	case stateShuttingDown:
		return false
	}
	return true
}

func (thread *phpRegularThread) onShutdown(){
    thread.state.set(stateDone)
}

func waitForScriptExecution(handler *phpRegularThread) string {
	select {
    case <-handler.thread.drainChan:
		logger.Info("drainChan", zap.Int("threadIndex", handler.thread.threadIndex))
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

func (thread *phpRegularThread) afterRequest(exitStatus int) {

	// if the request is nil, no script was executed
	if thread.activeRequest == nil {
		return
	}

	fc := thread.activeRequest.Context().Value(contextKey).(*FrankenPHPContext)
	fc.exitStatus = exitStatus
	maybeCloseContext(fc)
	thread.activeRequest = nil
}
