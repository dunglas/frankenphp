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
	isActive      bool                 // whether the thread is currently running
	isReady       bool                 // whether the thread is ready to accept requests
	threadIndex   int                  // the index of the thread in the phpThreads slice
	backoff       *exponentialBackoff  // backoff for worker failures
}

func (thread phpThread) getActiveRequest() *http.Request {
	if thread.workerRequest != nil {
		return thread.workerRequest
	}

	return thread.mainRequest
}

func (thread *phpThread) setReadyForRequests() {
	if thread.isReady {
		return
	}

    thread.isReady = true
    threadsReadyWG.Done()
    if thread.worker != nil {
        metrics.ReadyWorker(thread.worker.fileName)
    }
}


