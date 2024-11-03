package frankenphp

// #include <stdint.h>
// #include "frankenphp.h"
import "C"
import (
	"sync"
)

var (
	phpThreads           []*phpThread
	terminationWG        sync.WaitGroup
	mainThreadShutdownWG sync.WaitGroup
	threadsReadyWG       sync.WaitGroup
	shutdownWG           sync.WaitGroup
	done                 chan struct{}
)

// reserve a fixed number of PHP threads on the go side
func initPHPThreads(numThreads int) error {
	done = make(chan struct{})
	phpThreads = make([]*phpThread, numThreads)
	for i := 0; i < numThreads; i++ {
		phpThreads[i] = &phpThread{threadIndex: i}
	}
	return startMainThread(numThreads)
}

func drainPHPThreads() {
	close(done)
	shutdownWG.Wait()
	mainThreadShutdownWG.Done()
	terminationWG.Wait()
	phpThreads = nil
}

func startMainThread(numThreads int) error {
	threadsReadyWG.Add(1)
	mainThreadShutdownWG.Add(1)
	terminationWG.Add(1)
	if C.frankenphp_new_main_thread(C.int(numThreads)) != 0 {
		return MainThreadCreationError
	}
	threadsReadyWG.Wait()
	return nil
}

func getInactivePHPThread() *phpThread {
	for _, thread := range phpThreads {
		if !thread.isActive.Load() {
			return thread
		}
	}
	panic("not enough threads reserved")
}

//export go_frankenphp_main_thread_is_ready
func go_frankenphp_main_thread_is_ready() {
	threadsReadyWG.Done()
	mainThreadShutdownWG.Wait()
}

//export go_frankenphp_shutdown_main_thread
func go_frankenphp_shutdown_main_thread() {
	terminationWG.Done()
}
