package frankenphp

// #include "frankenphp.h"
import "C"
import (
	"net/http"
	"runtime"
	"unsafe"
)

type phpThread interface {
	onStartup()
    beforeScriptExecution() string
    afterScriptExecution(exitStatus int) bool
    onShutdown()
    pinner() *runtime.Pinner
    getActiveRequest() *http.Request
    getKnownVariableKeys() map[string]*C.zend_string
    setKnownVariableKeys(map[string]*C.zend_string)
}

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
	// right before the first work iteration
	onStartup func(*phpThread)
	// the actual work iteration (done in a loop)
	beforeScriptExecution func(*phpThread)
	// after the work iteration is done
	afterScriptExecution func(*phpThread, int)
	// after the thread is done
	onShutdown func(*phpThread)
	// exponential backoff for worker failures
	backoff *exponentialBackoff
	// known $_SERVER key names
	knownVariableKeys map[string]*C.zend_string
	// the state handler
	state *threadStateHandler
	stateMachine *workerStateMachine
}

func (thread *phpThread) getActiveRequest() *http.Request {
	if thread.workerRequest != nil {
		return thread.workerRequest
	}

	return thread.mainRequest
}

func (thread *phpThread) setActive(
	onStartup func(*phpThread),
	beforeScriptExecution func(*phpThread),
	afterScriptExecution func(*phpThread, int),
	onShutdown func(*phpThread),
) {
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
	thread.state.set(stateActive)
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

//export go_frankenphp_on_thread_startup
func go_frankenphp_on_thread_startup(threadIndex C.uintptr_t) {
	phpThreads[threadIndex].stateMachine.onStartup()
}

//export go_frankenphp_before_script_execution
func go_frankenphp_before_script_execution(threadIndex C.uintptr_t) *C.char {
	thread := phpThreads[threadIndex]
	scriptName := thread.stateMachine.beforeScriptExecution()
	// return the name of the PHP script that should be executed
	return thread.pinCString(scriptName)
}

//export go_frankenphp_after_script_execution
func go_frankenphp_after_script_execution(threadIndex C.uintptr_t, exitStatus C.int) {
	thread := phpThreads[threadIndex]
	if exitStatus < 0 {
		panic(ScriptExecutionError)
	}
	thread.stateMachine.afterScriptExecution(int(exitStatus))
	thread.Unpin()
}

//export go_frankenphp_on_thread_shutdown
func go_frankenphp_on_thread_shutdown(threadIndex C.uintptr_t) {
	thread.stateMachine.onShutdown()
}
