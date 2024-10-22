package frankenphp

// #include <stdint.h>
// #include <php_variables.h>
import "C"
import (
	"net/http"
	"runtime"
	"unsafe"
)

var phpThreads []*phpThread

type phpThread struct {
	runtime.Pinner

	mainRequest       *http.Request
	workerRequest     *http.Request
	worker            *worker
	knownVariableKeys map[string]*C.zend_string
}

func initPHPThreads(numThreads int) {
	phpThreads = make([]*phpThread, 0, numThreads)
	for i := 0; i < numThreads; i++ {
		phpThreads = append(phpThreads, &phpThread{})
	}
}

func (thread phpThread) getActiveRequest() *http.Request {
	if thread.workerRequest != nil {
		return thread.workerRequest
	}

	return thread.mainRequest
}

// Pin a string that is not null-terminated
func (thread *phpThread) pinString(s string) *C.char {
	sData := unsafe.StringData(s)
	thread.Pin(sData)
	return (*C.char)(unsafe.Pointer(sData))
}

// C strings must be null-terminated
func (thread *phpThread) pinCString(s string) *C.char {
	return thread.pinString(s+"\x00")
}
