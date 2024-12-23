package frankenphp

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

func (handler *regularThread) name() string {
	return "Regular PHP Thread"
}

func (handler *regularThread) waitForRequest() string {
	handler.state.markAsWaiting(true)

	var r *http.Request
	select {
	case <-handler.thread.drainChan:
		// go back to beforeScriptExecution
		return handler.beforeScriptExecution()
	case r = <-regularRequestChan:
	}

	handler.activeRequest = r
	handler.state.markAsWaiting(false)
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

	// if no thread was available, fan the request out to all threads

	select {
	case regularRequestChan <- r:
		// a thread was available to handle the request after all
		<-fc.done
		metrics.StopRequest()
		return
	default:
		// there is no thread available to handle the request
		metrics.QueuedRequest()
		go autoscaleRegularThreads()

		// block until the request is handled
		regularRequestChan <- r
		<-fc.done
		metrics.StopRequest()

		// success!
		metrics.DequeuedRequest()
		return
	}

}

func attachRegularThread(thread *phpThread) {
	regularThreadMu.Lock()
	regularThreads = append(regularThreads, thread)
	regularThreadMu.Unlock()
}

func detachRegularThread(thread *phpThread) {
	regularThreadMu.Lock()
	for i, t := range regularThreads {
		if t == thread {
			regularThreads = append(regularThreads[:i], regularThreads[i+1:]...)
			break
		}
	}
	regularThreadMu.Unlock()
}

func countRegularThreads() int {
	regularThreadMu.RLock()
	l := len(regularThreads)
	regularThreadMu.RUnlock()

	return l
}
