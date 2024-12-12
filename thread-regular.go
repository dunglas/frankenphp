package frankenphp

// #include "frankenphp.h"
import "C"
import (
	"net/http"
	"sync"
)

// representation of a non-worker PHP thread
// executes PHP scripts in a web context
// implements the threadHandler interface
type regularThread struct {
	state         *threadState
	thread        *phpThread
	activeRequest *http.Request
}

var (
	regularThreads     []*phpThread
	regularThreadMu    = &sync.RWMutex{}
	regularRequestChan chan *http.Request
)

func convertToRegularThread(thread *phpThread) {
	thread.setHandler(&regularThread{
		thread: thread,
		state:  thread.state,
	})
	attachRegularThread(thread)
}

// beforeScriptExecution returns the name of the script or an empty string on shutdown
func (handler *regularThread) beforeScriptExecution() string {
	switch handler.state.get() {
	case stateTransitionRequested:
		detachRegularThread(handler.thread)
		return handler.thread.transitionToNewHandler()
	case stateTransitionComplete:
		handler.state.set(stateReady)
		return handler.waitForRequest()
	case stateReady:
		return handler.waitForRequest()
	case stateShuttingDown:
		detachRegularThread(handler.thread)
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
	var r *http.Request
	select {
	case <-handler.thread.drainChan:
		// go back to beforeScriptExecution
		return handler.beforeScriptExecution()

	case r = <-handler.thread.requestChan:
	case r = <-regularRequestChan:
	}

	handler.activeRequest = r
	fc := r.Context().Value(contextKey).(*FrankenPHPContext)

	if err := updateServerContext(handler.thread, r, true, false); err != nil {
		rejectRequest(fc.responseWriter, err.Error())
		handler.afterRequest(0)
		handler.thread.Unpin()
		// go back to beforeScriptExecution
		return handler.beforeScriptExecution()
	}

	// set the scriptFilename that should be executed
	return fc.scriptFilename
}

func (handler *regularThread) afterRequest(exitStatus int) {
	fc := handler.activeRequest.Context().Value(contextKey).(*FrankenPHPContext)
	fc.exitStatus = exitStatus
	maybeCloseContext(fc)
	handler.activeRequest = nil
}

func handleRequestWithRegularPHPThreads(r *http.Request, fc *FrankenPHPContext) {
	metrics.StartRequest()
	regularThreadMu.RLock()

	// dispatch to all threads in order
	for _, thread := range regularThreads {
		select {
		case thread.requestChan <- r:
			regularThreadMu.RUnlock()
			<-fc.done
			metrics.StopRequest()
			return
		default:
			// thread is busy, continue
		}
	}
	regularThreadMu.RUnlock()

	// TODO: there can be possible auto-scaling here

	// if no thread was available, fan out to all threads
	select {
	case <-mainThread.done:
	case regularRequestChan <- r:
		<-fc.done
	}
	metrics.StopRequest()
}

func attachRegularThread(thread *phpThread) {
	regularThreadMu.Lock()
	defer regularThreadMu.Unlock()

	regularThreads = append(regularThreads, thread)
}

func detachRegularThread(thread *phpThread) {
	regularThreadMu.Lock()
	defer regularThreadMu.Unlock()

	for i, t := range regularThreads {
		if t == thread {
			regularThreads = append(regularThreads[:i], regularThreads[i+1:]...)
			break
		}
	}
}

func countRegularThreads() int {
	regularThreadMu.RLock()
	defer regularThreadMu.RUnlock()

	return len(regularThreads)
}
