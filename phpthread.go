package frankenphp

// #include "frankenphp.h"
import "C"
import (
	"fmt"
	"net/http"
	"runtime"
	"sync"
	"time"
	"unsafe"

	"go.uber.org/zap"
)

// representation of the actual underlying PHP thread
// identified by the index in the phpThreads slice
type phpThread struct {
	runtime.Pinner
	threadIndex       int
	knownVariableKeys map[string]*C.zend_string
	requestChan       chan *http.Request
	drainChan         chan struct{}
	handlerMu         *sync.Mutex
	handler           threadHandler
	state             *threadState
}

// interface that defines how the callbacks from the C thread should be handled
type threadHandler interface {
	name() string
	beforeScriptExecution() string
	afterScriptExecution(exitStatus int)
	getActiveRequest() *http.Request
}

func newPHPThread(threadIndex int) *phpThread {
	return &phpThread{
		threadIndex: threadIndex,
		requestChan: make(chan *http.Request),
		handlerMu:   &sync.Mutex{},
		state:       newThreadState(),
	}
}

// boot the underlying PHP thread
func (thread *phpThread) boot() {
	// thread must be in reserved state to boot
	if !thread.state.compareAndSwap(stateReserved, stateBooting) {
		logger.Error("thread is not in reserved state", zap.Int("threadIndex", thread.threadIndex), zap.Int("state", int(thread.state.get())))
		return
	}

	// boot threads as inactive
	thread.handlerMu.Lock()
	thread.handler = &inactiveThread{thread: thread}
	thread.drainChan = make(chan struct{})
	thread.handlerMu.Unlock()

	// start the actual posix thread - TODO: try this with go threads instead
	if !C.frankenphp_new_php_thread(C.uintptr_t(thread.threadIndex)) {
		logger.Panic("unable to create thread", zap.Int("threadIndex", thread.threadIndex))
	}
	thread.state.waitFor(stateInactive)
}

// shutdown the underlying PHP thread
func (thread *phpThread) shutdown() {
	if !thread.state.requestSafeStateChange(stateShuttingDown) {
		// already shutting down or done
		return
	}
	close(thread.drainChan)
	thread.state.waitFor(stateDone)
	thread.drainChan = make(chan struct{})

	// threads go back to the reserved state from which they can be booted again
	if mainThread.state.is(stateReady) {
		thread.state.set(stateReserved)
	}
}

// change the thread handler safely
// must be called from outside the PHP thread
func (thread *phpThread) setHandler(handler threadHandler) {
	thread.handlerMu.Lock()
	defer thread.handlerMu.Unlock()
	if !thread.state.requestSafeStateChange(stateTransitionRequested) {
		// no state change allowed == shutdown or done
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

// small status message for debugging
func (thread *phpThread) debugStatus() string {
	reqState := ""
	if waitTime := thread.state.waitTime(); waitTime > 0 {
		reqState = fmt.Sprintf(", waiting for %dms", waitTime)
	} else if r := thread.getActiveRequest(); r != nil {
		fc := r.Context().Value(contextKey).(*FrankenPHPContext)
		sinceMs := time.Since(fc.startedAt).Milliseconds()
		reqState = fmt.Sprintf(", handling %s for %dms ", fc.originalRequest.URL.Path, sinceMs)
	}
	return fmt.Sprintf("Thread %d (%s%s) %s", thread.threadIndex, thread.state.name(), reqState, thread.handler.name())
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

	// overwrite php.ini config (if necessary)
	mainThread.overridePHPIni()

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
	phpThreads[threadIndex].Unpin()
	phpThreads[threadIndex].state.set(stateDone)
}
