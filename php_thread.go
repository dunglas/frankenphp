package frankenphp

// #include "frankenphp.h"
import "C"
import (
	"net/http"
	"runtime"
	"unsafe"
)

// representation of the actual underlying PHP thread
// identified by the index in the phpThreads slice
type phpThread struct {
	runtime.Pinner

	threadIndex int
	knownVariableKeys map[string]*C.zend_string
	requestChan chan *http.Request
	drainChan chan struct{}
	handler	threadHandler
	state *stateHandler
}

// interface that defines how the callbacks from the C thread should be handled
type threadHandler interface {
	onStartup()
	beforeScriptExecution() string
	afterScriptExecution(exitStatus int) bool
	onShutdown()
	getActiveRequest() *http.Request
	isReadyToTransition() bool
}

func (thread *phpThread) getActiveRequest() *http.Request {
	return thread.handler.getActiveRequest()
}

func (thread *phpThread) setHandler(handler threadHandler) {
	thread.handler = handler
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
	phpThreads[threadIndex].handler.onStartup()
}

//export go_frankenphp_before_script_execution
func go_frankenphp_before_script_execution(threadIndex C.uintptr_t) *C.char {
	thread := phpThreads[threadIndex]
	scriptName := thread.handler.beforeScriptExecution()
	// return the name of the PHP script that should be executed
	return thread.pinCString(scriptName)
}

//export go_frankenphp_after_script_execution
func go_frankenphp_after_script_execution(threadIndex C.uintptr_t, exitStatus C.int) C.bool {
	thread := phpThreads[threadIndex]
	if exitStatus < 0 {
		panic(ScriptExecutionError)
	}
	shouldContinueExecution := thread.handler.afterScriptExecution(int(exitStatus))
	thread.Unpin()
	return C.bool(shouldContinueExecution)
}

//export go_frankenphp_on_thread_shutdown
func go_frankenphp_on_thread_shutdown(threadIndex C.uintptr_t) {
	phpThreads[threadIndex].handler.onShutdown()
}
