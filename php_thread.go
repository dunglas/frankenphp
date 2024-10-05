package frankenphp

import (
	"net/http"
	"runtime"
)

var phpThreads []*PHPThread

type PHPThread struct {
	threadIndex int
	mainRequest *http.Request
	workerRequest *http.Request
	pinner *runtime.Pinner
	worker *worker
}

func initializePhpThreads(numThreads int) {
	phpThreads = make([]*PHPThread, numThreads)
	for i := 0; i < numThreads; i++ {
		phpThreads[i] = &PHPThread{threadIndex: i, pinner: &runtime.Pinner{}}
	}
}

func getPHPThread(threadIndex int) *PHPThread {
	if(threadIndex >= 0 && threadIndex < len(phpThreads)) {
		return phpThreads[threadIndex]
	}
	panic("no such thread")
}

func (thread *PHPThread) setMainRequest(request *http.Request) {
	thread.mainRequest = request
}

func (thread *PHPThread) setWorkerRequest(request *http.Request) {
	thread.workerRequest = request
}

func (thread *PHPThread) getMainRequest() *http.Request {
	if(thread.mainRequest != nil) {
        return thread.mainRequest
    }
    panic("no worker request")
}

func (thread *PHPThread) getActiveRequest() *http.Request {
	if(thread.workerRequest != nil) {
		return thread.workerRequest
	}
	if(thread.mainRequest != nil) {
		return thread.mainRequest
	}
	panic("no active request")
}