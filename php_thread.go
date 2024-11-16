package frankenphp

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

	// the script name for the current request
	scriptName string
	// the index in the phpThreads slice
	threadIndex int
	// whether the thread has work assigned to it
	isActive atomic.Bool
	// whether the thread is ready for work
	isReady atomic.Bool
	// right before the first work iteration
	onStartup func(*phpThread)
	// the actual work iteration (done in a loop)
	beforeScriptExecution func(*phpThread)
	// after the work iteration is done
	afterScriptExecution func(*phpThread, int)
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
	thread.scriptName = ""
	thread.beforeScriptExecution = func(thread *phpThread) {
		thread.requestChan = make(chan *http.Request)
		select {
		case <-done:
		case <-thread.done:
		}
	}
}

func (thread *phpThread) setActive(
	onStartup func(*phpThread),
	beforeScriptExecution func(*phpThread),
	afterScriptExecution func(*phpThread, int),
	onShutdown func(*phpThread),
) {
	thread.isActive.Store(true)

	// to avoid race conditions, the thread sets its own hooks on startup
	thread.onStartup = func(thread *phpThread) {
		if thread.onShutdown != nil {
			thread.onShutdown(thread)
		}
		thread.onStartup = onStartup
		thread.beforeScriptExecution = beforeScriptExecution
		thread.onShutdown = onShutdown
		thread.afterScriptExecution = afterScriptExecution
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

//export go_frankenphp_before_script_execution
func go_frankenphp_before_script_execution(threadIndex C.uintptr_t) *C.char {
	// returning nil signals the thread to stop
	if threadsAreDone.Load() {
		return nil
	}
	thread := phpThreads[threadIndex]

	// if the thread is not ready, set it up
	if thread.isReady.CompareAndSwap(false, true) {
		thread.done = make(chan struct{})
		if thread.onStartup != nil {
			thread.onStartup(thread)
		}
		if threadsAreBooting.Load() {
			threadsReadyWG.Done()
			threadsReadyWG.Wait()
		}
	}

	// execute a hook before the script is executed
	thread.beforeScriptExecution(thread)

	// return the name of the PHP script that should be executed
	return thread.pinCString(thread.scriptName)
}

//export go_frankenphp_after_script_execution
func go_frankenphp_after_script_execution(threadIndex C.uintptr_t, exitStatus C.int) {
	thread := phpThreads[threadIndex]
	if exitStatus < 0 {
		panic(ScriptExecutionError)
	}
	if thread.afterScriptExecution != nil {
		thread.afterScriptExecution(thread, int(exitStatus))
	}
	thread.Unpin()
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
