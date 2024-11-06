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

	mainRequest   *http.Request
	workerRequest *http.Request
	requestChan   chan *http.Request
	worker        *worker

	// the index in the phpThreads slice
	threadIndex int
	// whether the thread has work assigned to it
	isActive atomic.Bool
	// whether the thread is ready for work
	isReady atomic.Bool
	// right before the first work iteration
	onStartup func(*phpThread)
	// the actual work iteration (done in a loop)
	onWork func(*phpThread)
	// after the thread is done
	onShutdown func(*phpThread)
	// chan to signal the thread to stop the current work iteration
	done chan struct{}
	// exponential backoff for worker failures
	backoff *exponentialBackoff
	// known $_SERVER key names
	knownVariableKeys map[string]*C.zend_string
}

func (thread *phpThread) getActiveRequest() *http.Request {
	if thread.workerRequest != nil {
		return thread.workerRequest
	}

	return thread.mainRequest
}

// TODO: Also consider this case: work => inactive => work
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

func (thread *phpThread) setActive(onStartup func(*phpThread), onWork func(*phpThread), onShutdown func(*phpThread)) {
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

	// signal to the thread to stop it's current execution and call the onStartup hook
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
		if threadsAreBooting.Load() {
			threadsReadyWG.Done()
			threadsReadyWG.Wait()
		}
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
