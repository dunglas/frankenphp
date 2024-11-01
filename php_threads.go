package frankenphp

// #include <stdint.h>
// #include "frankenphp.h"
import "C"
import (
	"fmt"
	"sync"
)

var (
	phpThreads []*phpThread
	mainThreadWG sync.WaitGroup
	terminationWG sync.WaitGroup
	mainThreadShutdownWG sync.WaitGroup
	readyWG sync.WaitGroup
	shutdownWG  sync.WaitGroup
)

// reserve a fixed number of PHP threads on the go side
func initPHPThreads(numThreads int) error {
	phpThreads = make([]*phpThread, numThreads)
	for i := 0; i < numThreads; i++ {
		phpThreads[i] = &phpThread{threadIndex: i}
	}
	return startMainThread(numThreads)
}

func drainPHPThreads() {
	close(done)
	shutdownWG.Wait()
	phpThreads = nil
	mainThreadShutdownWG.Done()
	terminationWG.Wait()
}

func startMainThread(numThreads int) error {
	mainThreadWG.Add(1)
	mainThreadShutdownWG.Add(1)
	terminationWG.Add(1)
    if C.frankenphp_new_main_thread(C.int(numThreads)) != 0 {
        return MainThreadCreationError
    }
    mainThreadWG.Wait()
    return nil
}

func startNewPHPThread() error {
	readyWG.Add(1)
	shutdownWG.Add(1)
	thread := getInactiveThread()
	thread.isActive = true
	if C.frankenphp_new_php_thread(C.uintptr_t(thread.threadIndex)) != 0 {
		return fmt.Errorf("error creating thread %d", thread.threadIndex)
	}
	return nil
}

func startNewWorkerThread(worker *worker) error {
	workersReadyWG.Add(1)
    workerShutdownWG.Add(1)
	thread := getInactiveThread()
    thread.worker = worker
    thread.isActive = true
    if C.frankenphp_new_worker_thread(C.uintptr_t(thread.threadIndex)) != 0 {
        return fmt.Errorf("failed to create worker thread")
    }

	return nil
}

func getInactiveThread() *phpThread {
	for _, thread := range phpThreads {
		if !thread.isActive {
			return thread
		}
	}

	return nil
}

//export go_main_thread_is_ready
func go_main_thread_is_ready(){
	mainThreadWG.Done()
	mainThreadShutdownWG.Wait()
}

//export go_shutdown_main_thread
func go_shutdown_main_thread(){
	terminationWG.Done()
}

//export go_shutdown_php_thread
func go_shutdown_php_thread(threadIndex C.uintptr_t) {
	thread := phpThreads[threadIndex]
	thread.Unpin()
	thread.isActive = false
	shutdownWG.Done()
}

//export go_shutdown_worker_thread
func go_shutdown_worker_thread(threadIndex C.uintptr_t) {
	thread := phpThreads[threadIndex]
	thread.Unpin()
	thread.isActive = false
	thread.worker = nil
    workerShutdownWG.Done()
}