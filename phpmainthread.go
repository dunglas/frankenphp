package frankenphp

// #include "frankenphp.h"
import "C"
import (
	"fmt"
	"sync"
)

// represents the main PHP thread
// the thread needs to keep running as long as all other threads are running
type phpMainThread struct {
	state      *threadState
	done       chan struct{}
	numThreads int
}

var (
	phpThreads []*phpThread
	mainThread *phpMainThread
)

// start the main PHP thread
// start a fixed number of inactive PHP threads
// reserve a fixed number of possible PHP threads
func initPHPThreads(numThreads int, numMaxThreads int) error {
	mainThread = &phpMainThread{
		state:      newThreadState(),
		done:       make(chan struct{}),
		numThreads: numThreads,
	}
	phpThreads = make([]*phpThread, numMaxThreads)

	if err := mainThread.start(); err != nil {
		return err
	}

	// initialize all threads as inactive
	for i := 0; i < numMaxThreads; i++ {
		phpThreads[i] = newPHPThread(i)
		convertToInactiveThread(phpThreads[i])
	}

	// start the underlying C threads
	ready := sync.WaitGroup{}
	ready.Add(numThreads)
	for i := 0; i < numThreads; i++ {
		thread := phpThreads[i]
		go func() {
			thread.boot()
			ready.Done()
		}()
	}
	ready.Wait()

	return nil
}

func ThreadDebugStatus() string {
	statusMessage := ""
	reservedThreadCount := 0
	for _, thread := range phpThreads {
		if thread.state.is(stateReserved) {
			reservedThreadCount++
			continue
		}
		statusMessage += thread.debugStatus() + "\n"
	}
	statusMessage += fmt.Sprintf("%d additional threads can be started at runtime\n", reservedThreadCount)
	return statusMessage
}

func drainPHPThreads() {
	doneWG := sync.WaitGroup{}
	doneWG.Add(len(phpThreads))
	for _, thread := range phpThreads {
		if thread.state.is(stateReserved) {
			doneWG.Done()
			continue
		}
		thread.handlerMu.Lock()
		_ = thread.state.requestSafeStateChange(stateShuttingDown)
		close(thread.drainChan)
	}
	close(mainThread.done)
	for _, thread := range phpThreads {
		if thread.state.is(stateReserved) {
			continue
		}
		go func(thread *phpThread) {
			thread.state.waitFor(stateDone)
			thread.handlerMu.Unlock()
			doneWG.Done()
		}(thread)
	}
	doneWG.Wait()
	mainThread.state.set(stateShuttingDown)
	mainThread.state.waitFor(stateDone)
	phpThreads = nil
}

func (mainThread *phpMainThread) start() error {
	if C.frankenphp_new_main_thread(C.int(mainThread.numThreads)) != 0 {
		return MainThreadCreationError
	}
	mainThread.state.waitFor(stateReady)
	return nil
}

func getInactivePHPThread() *phpThread {
	thread := getPHPThreadAtState(stateInactive)
	if thread != nil {
		return thread
	}
	thread = getPHPThreadAtState(stateReserved)
	if thread == nil {
		return nil
	}
	thread.boot()
	return thread
}

func getPHPThreadAtState(state stateID) *phpThread {
	for _, thread := range phpThreads {
		if thread.state.is(state) {
			return thread
		}
	}
	return nil
}

//export go_frankenphp_main_thread_is_ready
func go_frankenphp_main_thread_is_ready() {
	mainThread.state.set(stateReady)
	mainThread.state.waitFor(stateShuttingDown)
}

//export go_frankenphp_shutdown_main_thread
func go_frankenphp_shutdown_main_thread() {
	mainThread.state.set(stateDone)
}
