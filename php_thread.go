package frankenphp

// #include <stdint.h>
import "C"
import (
	"net/http"
	"runtime"
)

var phpThreads []*phpThread

type phpThread struct {
	runtime.Pinner

	mainRequest   *http.Request
	workerRequest *http.Request
	worker        *worker
	isActive	  bool
	isReady	      bool
	threadIndex   int
}

func initPHPThreads(numThreads int) {
	phpThreads = make([]*phpThread, 0, numThreads)
	for i := 0; i < numThreads; i++ {
		phpThreads = append(phpThreads, &phpThread{threadIndex: i})
	}
}

func getInactiveThread() *phpThread {
	for _, thread := range phpThreads {
		if !thread.isActive {
			return thread
		}
	}

	return nil
}

func (thread phpThread) getActiveRequest() *http.Request {
	if thread.workerRequest != nil {
		return thread.workerRequest
	}

	return thread.mainRequest
}
