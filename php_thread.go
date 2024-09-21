package frankenphp

import (
	"net/http"
	"sync"
	"go.uber.org/zap"
	"runtime"
)

var phpThreads sync.Map

type PHPThread struct {
	threadId int
	mainRequest *http.Request
	workerRequest *http.Request
	pinner *runtime.Pinner
}

func getPHPThread(threadId int) *PHPThread {
	if thread, ok := phpThreads.Load(threadId); ok {
		return thread.(*PHPThread)
	}
	thread := &PHPThread{threadId: threadId, pinner: &runtime.Pinner{}}
	phpThreads.Store(threadId, thread)
	logger.Debug("new php thread registered", zap.Int("threadId", threadId))
	return thread
}

func (thread *PHPThread) setMainRequest(request *http.Request) {
	thread.mainRequest = request
}

func (thread *PHPThread) setWorkerRequest(request *http.Request) {
	thread.workerRequest = request
}

func (thread *PHPThread) getWorkerRequest() *http.Request {
	if(thread.workerRequest != nil) {
		return thread.workerRequest
	}
	panic("no worker request")
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