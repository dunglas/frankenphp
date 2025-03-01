package frankenphp

import (
	"sync"
)

// representation of a non-worker PHP thread
// executes PHP scripts in a web context
// implements the threadHandler interface
type regularThread struct {
	state          *threadState
	thread         *phpThread
	requestContext *frankenPHPContext
}

var (
	regularThreads     []*phpThread
	regularThreadMu    = &sync.RWMutex{}
	regularRequestChan chan *frankenPHPContext
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

func (handler *regularThread) getRequestContext() *frankenPHPContext {
	return handler.requestContext
}

func (handler *regularThread) name() string {
	return "Regular PHP Thread"
}

func (handler *regularThread) waitForRequest() string {
	// clear any previously sandboxed env
	clearSandboxedEnv(handler.thread)

	handler.state.markAsWaiting(true)

	var fc *frankenPHPContext
	select {
	case <-handler.thread.drainChan:
		// go back to beforeScriptExecution
		return handler.beforeScriptExecution()
	case fc = <-regularRequestChan:
	}

	handler.requestContext = fc
	handler.state.markAsWaiting(false)

	if err := updateServerContext(handler.thread, fc, false); err != nil {
		fc.rejectBadRequest(err.Error())
		handler.afterRequest(0)
		handler.thread.Unpin()
		// go back to beforeScriptExecution
		return handler.beforeScriptExecution()
	}

	// set the scriptFilename that should be executed
	return fc.scriptFilename
}

func (handler *regularThread) afterRequest(exitStatus int) {
	fc := handler.requestContext
	fc.exitStatus = exitStatus
	fc.closeContext()
	handler.requestContext = nil
}

func handleRequestWithRegularPHPThreads(fc *frankenPHPContext) {
	metrics.StartRequest()
	select {
	case regularRequestChan <- fc:
		// a thread was available to handle the request immediately
		<-fc.done
		metrics.StopRequest()
		return
	default:
		// no thread was available
	}

	// if no thread was available, mark the request as queued and fan it out to all threads
	metrics.QueuedRequest()
	for {
		select {
		case regularRequestChan <- fc:
			metrics.DequeuedRequest()
			<-fc.done
			metrics.StopRequest()
			return
		case scaleChan <- fc:
			// the request has triggered scaling, continue to wait for a thread
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
