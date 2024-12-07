package frankenphp

// #include "frankenphp.h"
import "C"
import (
	"fmt"
	"sync"
)

var (
	phpThreads      []*phpThread
	done            chan struct{}
	mainThreadState *threadState
)

// reserve a fixed number of PHP threads on the go side
func initPHPThreads(numThreads int) error {
	done = make(chan struct{})
	phpThreads = make([]*phpThread, numThreads)
	for i := 0; i < numThreads; i++ {
		phpThreads[i] = &phpThread{
			threadIndex: i,
			drainChan:   make(chan struct{}),
			state:       newThreadState(),
		}
		convertToInactiveThread(phpThreads[i])
	}
	if err := startMainThread(numThreads); err != nil {
		return err
	}

	// initialize all threads as inactive
	ready := sync.WaitGroup{}
	ready.Add(len(phpThreads))

	for _, thread := range phpThreads {
		go func() {
			if !C.frankenphp_new_php_thread(C.uintptr_t(thread.threadIndex)) {
				panic(fmt.Sprintf("unable to create thread %d", thread.threadIndex))
			}
			thread.state.waitFor(stateInactive)
			ready.Done()
		}()
	}

	ready.Wait()

	return nil
}

func drainPHPThreads() {
	doneWG := sync.WaitGroup{}
	doneWG.Add(len(phpThreads))
	for _, thread := range phpThreads {
		thread.state.set(stateShuttingDown)
		close(thread.drainChan)
	}
	close(done)
	for _, thread := range phpThreads {
		go func(thread *phpThread) {
			thread.state.waitFor(stateDone)
			doneWG.Done()
		}(thread)
	}
	doneWG.Wait()
	mainThreadState.set(stateShuttingDown)
	mainThreadState.waitFor(stateDone)
	phpThreads = nil
}

func startMainThread(numThreads int) error {
	mainThreadState = newThreadState()
	if C.frankenphp_new_main_thread(C.int(numThreads)) != 0 {
		return MainThreadCreationError
	}
	mainThreadState.waitFor(stateActive)
	return nil
}

func getInactivePHPThread() *phpThread {
	for _, thread := range phpThreads {
		if thread.handler.isReadyToTransition() {
			return thread
		}
	}
	panic("not enough threads reserved")
}

//export go_frankenphp_main_thread_is_ready
func go_frankenphp_main_thread_is_ready() {
	mainThreadState.set(stateActive)
	mainThreadState.waitFor(stateShuttingDown)
}

//export go_frankenphp_shutdown_main_thread
func go_frankenphp_shutdown_main_thread() {
	mainThreadState.set(stateDone)
}
