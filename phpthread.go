package frankenphp

// #include "frankenphp.h"
import "C"
import (
	"net/http"
	"runtime"
	"sync"
	"unsafe"
)

// representation of the actual underlying PHP thread
// identified by the index in the phpThreads slice
type phpThread struct {
	runtime.Pinner

	threadIndex int
	requestChan chan *http.Request
	drainChan   chan struct{}
	handlerMu   *sync.Mutex
	handler     threadHandler
	state       *threadState
}

// interface that defines how the callbacks from the C thread should be handled
type threadHandler interface {
	beforeScriptExecution() string
	afterScriptExecution(exitStatus int)
	getActiveRequest() *http.Request
}

func newPHPThread(threadIndex int) *phpThread {
	return &phpThread{
		threadIndex: threadIndex,
		drainChan:   make(chan struct{}),
		requestChan: make(chan *http.Request),
		handlerMu:   &sync.Mutex{},
		state:       newThreadState(),
	}
}

// change the thread handler safely
// must be called from outside the PHP thread
func (thread *phpThread) setHandler(handler threadHandler) {
	logger.Debug("setHandler")
	thread.handlerMu.Lock()
	defer thread.handlerMu.Unlock()
	if !thread.state.requestSafeStateChange(stateTransitionRequested) {
		// no state change allowed == shutdown
		return
	}
	close(thread.drainChan)
	thread.state.waitFor(stateTransitionInProgress)
	thread.handler = handler
	thread.drainChan = make(chan struct{})
	thread.state.set(stateTransitionComplete)
}

// transition to a new handler safely
// is triggered by setHandler and executed on the PHP thread
func (thread *phpThread) transitionToNewHandler() string {
	thread.state.set(stateTransitionInProgress)
	thread.state.waitFor(stateTransitionComplete)
	// execute beforeScriptExecution of the new handler
	return thread.handler.beforeScriptExecution()
}

func (thread *phpThread) getActiveRequest() *http.Request {
	return thread.handler.getActiveRequest()
}

// Pin a string that is not null-terminated
// PHP's zend_string may contain null-bytes
func (thread *phpThread) pinString(s string) *C.char {
	sData := unsafe.StringData(s)
	if sData == nil {
		return nil
	}
	thread.Pin(sData)

	return (*C.char)(unsafe.Pointer(sData))
}

// C strings must be null-terminated
func (thread *phpThread) pinCString(s string) *C.char {
	return thread.pinString(s + "\x00")
}

//export go_frankenphp_before_script_execution
func go_frankenphp_before_script_execution(threadIndex C.uintptr_t) *C.char {
	thread := phpThreads[threadIndex]
	scriptName := thread.handler.beforeScriptExecution()

	// if no scriptName is passed, shut down
	if scriptName == "" {
		return nil
	}
	// return the name of the PHP script that should be executed
	return thread.pinCString(scriptName)
}

//export go_frankenphp_after_script_execution
func go_frankenphp_after_script_execution(threadIndex C.uintptr_t, exitStatus C.int) {
	thread := phpThreads[threadIndex]
	if exitStatus < 0 {
		panic(ScriptExecutionError)
	}
	thread.handler.afterScriptExecution(int(exitStatus))

	// unpin all memory used during script execution
	thread.Unpin()
}

//export go_frankenphp_on_thread_shutdown
func go_frankenphp_on_thread_shutdown(threadIndex C.uintptr_t) {
	thread := phpThreads[threadIndex]
	thread.Unpin()
	thread.state.set(stateDone)
}
