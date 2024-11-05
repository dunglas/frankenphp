package frankenphp

// #include <stdint.h>
// #include <stdbool.h>
// #include <php_variables.h>
// #include "frankenphp.h"
import "C"
import (
	"net/http"
	"runtime"
	"sync/atomic"
	"unsafe"
)

type phpThread struct {
	runtime.Pinner

	mainRequest       *http.Request
	workerRequest     *http.Request
	worker            *worker
	requestChan       chan *http.Request
	done              chan struct{}       // to signal the thread to stop the
	threadIndex       int                 // the index of the thread in the phpThreads slice
	isActive          atomic.Bool         // whether the thread is currently running
	isReady           atomic.Bool         // whether the thread is ready for work
	onStartup         func(*phpThread)    // the function to run when ready
	onWork            func(*phpThread)    // the function to run in a loop when ready
	onShutdown        func(*phpThread)    // the function to run after shutdown
	backoff           *exponentialBackoff // backoff for worker failures
	knownVariableKeys map[string]*C.zend_string
}

func (thread phpThread) getActiveRequest() *http.Request {
	if thread.workerRequest != nil {
		return thread.workerRequest
	}

	return thread.mainRequest
}

func (thread *phpThread) setInactive() {
	thread.isActive.Store(false)
	thread.onWork = func(thread *phpThread) {
		thread.requestChan = make(chan *http.Request)
		select {
		case <-done:
		case <-thread.done:
		}
	}
}

func (thread *phpThread) setHooks(onStartup func(*phpThread), onWork func(*phpThread), onShutdown func(*phpThread)) {
	thread.isActive.Store(true)

	// to avoid race conditions, the thread sets its own hooks on startup
	thread.onStartup = func(thread *phpThread) {
		if thread.onShutdown != nil {
			thread.onShutdown(thread)
		}
		thread.onStartup = onStartup
		thread.onWork = onWork
		thread.onShutdown = onShutdown
		if thread.onStartup != nil {
			thread.onStartup(thread)
		}
	}

	threadsReadyWG.Add(1)
	close(thread.done)
	thread.isReady.Store(false)
}

// Pin a string that is not null-terminated
// PHP's zend_string may contain null-bytes
func (thread *phpThread) pinString(s string) *C.char {
	sData := unsafe.StringData(s)
	thread.Pin(sData)
	return (*C.char)(unsafe.Pointer(sData))
}

// C strings must be null-terminated
func (thread *phpThread) pinCString(s string) *C.char {
	return thread.pinString(s + "\x00")
}

//export go_frankenphp_on_thread_work
func go_frankenphp_on_thread_work(threadIndex C.uintptr_t) C.bool {
	// first check if FrankPHP is shutting down
	if threadsAreDone.Load() {
		return C.bool(false)
	}
	thread := phpThreads[threadIndex]

	// if the thread is not ready, set it up
	if !thread.isReady.Load() {
		thread.isReady.Store(true)
		thread.done = make(chan struct{})
		if thread.onStartup != nil {
			thread.onStartup(thread)
		}
		threadsReadyWG.Done()
	}

	// do the actual work
	thread.onWork(thread)
	return C.bool(true)
}

//export go_frankenphp_on_thread_shutdown
func go_frankenphp_on_thread_shutdown(threadIndex C.uintptr_t) {
	thread := phpThreads[threadIndex]
	thread.Unpin()
	if thread.onShutdown != nil {
		thread.onShutdown(thread)
	}
	shutdownWG.Done()
}
