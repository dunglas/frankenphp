package frankenphp

import (
	"sync"
)

// EXPERIMENTAL (only used in tests)
// representation of a thread that handles tasks directly assigned by go
// implements the threadHandler interface
type taskThread struct {
	thread   *phpThread
	execChan chan *task
}

// task callbacks will be executed directly on the PHP thread
// therefore having full access to the PHP runtime
type task struct {
	callback func()
	done     sync.Mutex
}

func newTask(cb func()) *task {
	t := &task{callback: cb}
	t.done.Lock()

	return t
}

func (t *task) waitForCompletion() {
	t.done.Lock()
}

func convertToTaskThread(thread *phpThread) *taskThread {
	handler := &taskThread{
		thread:   thread,
		execChan: make(chan *task),
	}
	thread.setHandler(handler)
	return handler
}

func (handler *taskThread) beforeScriptExecution() string {
	thread := handler.thread

	switch thread.state.get() {
	case stateTransitionRequested:
		return thread.transitionToNewHandler()
	case stateBooting, stateTransitionComplete:
		thread.state.set(stateReady)
		handler.waitForTasks()

		return handler.beforeScriptExecution()
	case stateReady:
		handler.waitForTasks()

		return handler.beforeScriptExecution()
	case stateShuttingDown:
		// signal to stop
		return ""
	}
	panic("unexpected state: " + thread.state.name())
}

func (handler *taskThread) afterScriptExecution(int) {
	panic("task threads should not execute scripts")
}

func (handler *taskThread) getRequestContext() *frankenPHPContext {
	return nil
}

func (handler *taskThread) name() string {
	return "Task PHP Thread"
}

func (handler *taskThread) waitForTasks() {
	for {
		select {
		case task := <-handler.execChan:
			task.callback()
			task.done.Unlock() // unlock the task to signal completion
		case <-handler.thread.drainChan:
			// thread is shutting down, do not execute the function
			return
		}
	}
}

func (handler *taskThread) execute(t *task) {
	handler.execChan <- t
}
