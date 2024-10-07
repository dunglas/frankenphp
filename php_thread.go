package frankenphp

import (
	"net/http"
	"runtime"
)

var phpThreads []*phpThread

type phpThread struct {
	threadIndex   int
	mainRequest   *http.Request
	workerRequest *http.Request
	pinner        *runtime.Pinner
	worker        *worker
}

func initializePHPThreads(numThreads int) {
	phpThreads = make([]*phpThread, numThreads)
	for i := 0; i < numThreads; i++ {
		phpThreads[i] = &phpThread{threadIndex: i, pinner: &runtime.Pinner{}}
	}
}

func getPHPThread(threadIndex int) *phpThread {
	if threadIndex >= 0 && threadIndex < len(phpThreads) {
		return phpThreads[threadIndex]
	}
	panic("no such thread")
}

func (thread *phpThread) setMainRequest(request *http.Request) {
	thread.mainRequest = request
}

func (thread *phpThread) setWorkerRequest(request *http.Request) {
	thread.workerRequest = request
}

func (thread *phpThread) getMainRequest() *http.Request {
	if thread.mainRequest != nil {
		return thread.mainRequest
	}
	panic("no worker request")
}

func (thread *phpThread) getActiveRequest() *http.Request {
	if thread.workerRequest != nil {
		return thread.workerRequest
	}
	if thread.mainRequest != nil {
		return thread.mainRequest
	}
	panic("no active request")
}
