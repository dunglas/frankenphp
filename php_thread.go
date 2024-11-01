package frankenphp

// #include <stdint.h>
import "C"
import (
	"net/http"
	"runtime"
)

type phpThread struct {
	runtime.Pinner

	mainRequest   *http.Request
	workerRequest *http.Request
	worker        *worker
	isActive	  bool
	isReady	      bool
	threadIndex   int
}

func (thread phpThread) getActiveRequest() *http.Request {
	if thread.workerRequest != nil {
		return thread.workerRequest
	}

	return thread.mainRequest
}
