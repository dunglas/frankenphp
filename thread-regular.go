package frankenphp

import (
	"net/http"
	"sync"
	"time"
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
	select {
	case regularRequestChan <- r:
		// a thread was available to handle the request immediately
		<-fc.done
		metrics.StopRequest()
		return
	default:
		// no thread was available
	}

	// if no thread was available, fan the request out to all threads
	// if a request has waited for too long, trigger autoscaling

	timeout := allowedStallTime
	timer := time.NewTimer(timeout)
	metrics.QueuedRequest()

	for {
		select {
		case regularRequestChan <- r:
			// a thread was available to handle the request after all
			timer.Stop()
			metrics.DequeuedRequest()
			<-fc.done
			metrics.StopRequest()
			return
		case <-timer.C:
			// reaching here means we might not have spawned enough threads
			requestNewRegularThread()

			// TODO: reject a request that has been waiting for too long (504)
			// TODO: limit the amount of stalled requests (maybe) (503)

			// re-trigger autoscaling with an exponential backoff
			timeout *= 2
			timer.Reset(timeout)
		}
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
