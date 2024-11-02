package frankenphp

// #include <stdint.h>
// #include <stdbool.h>
// #include "frankenphp.h"
import "C"
import (
	"fmt"
	"net/http"
	"runtime"
)

type phpThread struct {
	runtime.Pinner

	mainRequest   *http.Request
	workerRequest *http.Request
	worker        *worker
	isActive      bool                  // whether the thread is currently running
	isReady       bool                  // whether the thread is ready to accept requests
	threadIndex   int                   // the index of the thread in the phpThreads slice
	onStartup     func(*phpThread)      // the function to run on startup
	onWork        func(*phpThread) bool // the function to run in the thread
	onShutdown    func(*phpThread)      // the function to run after shutdown
	backoff       *exponentialBackoff   // backoff for worker failures
}

func (thread phpThread) getActiveRequest() *http.Request {
	if thread.workerRequest != nil {
		return thread.workerRequest
	}

	return thread.mainRequest
}

func (thread *phpThread) run() error {
	if thread.isActive {
		return fmt.Errorf("thread is already running %d", thread.threadIndex)
	}
	threadsReadyWG.Add(1)
	shutdownWG.Add(1)
	thread.isActive = true
	if C.frankenphp_new_php_thread(C.uintptr_t(thread.threadIndex)) != 0 {
		return fmt.Errorf("error creating thread %d", thread.threadIndex)
	}
	return nil
}

func (thread *phpThread) setReady() {
	if thread.isReady {
		return
	}

	thread.isReady = true
	threadsReadyWG.Done()
	if thread.worker != nil {
		metrics.ReadyWorker(thread.worker.fileName)
	}
}

//export go_frankenphp_on_thread_startup
func go_frankenphp_on_thread_startup(threadIndex C.uintptr_t) {
	thread := phpThreads[threadIndex]
	thread.isReady = true
	if thread.onStartup != nil {
		thread.onStartup(thread)
	}
	threadsReadyWG.Done()
}

//export go_frankenphp_on_thread_work
func go_frankenphp_on_thread_work(threadIndex C.uintptr_t) C.bool {
	thread := phpThreads[threadIndex]
	return C.bool(thread.onWork(thread))
}

//export go_frankenphp_on_thread_shutdown
func go_frankenphp_on_thread_shutdown(threadIndex C.uintptr_t) {
	thread := phpThreads[threadIndex]
	thread.isActive = false
	thread.isReady = false
	thread.Unpin()
	if thread.onShutdown != nil {
		thread.onShutdown(thread)
	}
	shutdownWG.Done()
}
