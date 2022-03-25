package frankenphp

import (
	"C"
	"net/http"
	"sync"
	"sync/atomic"
)

var workers sync.Map

type worker struct {
	pool sync.Pool
	in   chan *http.Request
}

func getWorker(fileName string) *worker {
	var pool sync.Pool
	p, ok := workers.Load(fileName)
	if ok {
		pool = p.(sync.Pool)
	} else {
		pool = sync.Pool{}
		pool.New = func() interface{} {
			return newWorker(fileName, pool)
		}

		workers.Store(fileName, pool)
	}

	return pool.Get().(*worker)
}

func WorkerHandleRequest(responseWriter http.ResponseWriter, request *http.Request) error {
	if atomic.LoadInt32(&started) < 1 {
		if err := Startup(); err != nil {
			return err
		}
	}

	// todo: refactor to not call populateEnv twice
	_, err := populateEnv(request)
	if err != nil {
		return err
	}

	fc := request.Context().Value(FrankenPHPContextKey).(*FrankenPHPContext)
	fc.responseWriter = responseWriter
	fc.done = make(chan interface{})

	worker := getWorker(fc.Env["SCRIPT_FILENAME"])

	worker.in <- request
	<-fc.done

	return nil
}
