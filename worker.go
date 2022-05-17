package frankenphp

import (
	"C"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
)
import (
	"runtime/cgo"
)

var requestsChans sync.Map // map[fileName]cgo.NewHandle(chan *http.Request)
var workersWaitGroup sync.WaitGroup

func WorkerHandleRequest(responseWriter http.ResponseWriter, request *http.Request) error {
	if atomic.LoadInt32(&started) < 1 {
		panic("FrankenPHP isn't started, call frankenphp.Startup()")
	}

	if err := populateEnv(request); err != nil {
		return err
	}

	fc, _ := FromContext(request.Context())
	v, ok := requestsChans.Load(fc.Env["SCRIPT_FILENAME"])
	if !ok {
		panic(fmt.Errorf("No worker started for script %s", fc.Env["SCRIPT_FILENAME"]))
	}
	rch := v.(cgo.Handle)
	rc := rch.Value().(chan *http.Request)

	fc.responseWriter = responseWriter
	fc.done = make(chan interface{})
	fc.Env["FRANKENPHP_WORKER"] = "1"

	rc <- request
	<-fc.done

	return nil
}

func StartWorkers(fileName string, nbWorkers int) {
	if _, ok := requestsChans.Load(fileName); ok {
		panic(fmt.Errorf("workers %q: already started", fileName))
	}

	rc := make(chan *http.Request)
	rch := cgo.NewHandle(rc)

	requestsChans.Store(fileName, rch)

	for i := 0; i < nbWorkers; i++ {
		newWorker(fileName, rch)
	}
}

func StopWorkers() {
	requestsChans.Range(func(k, v any) bool {
		close(v.(cgo.Handle).Value().(chan *http.Request))
		requestsChans.Delete(k)

		return true
	})

	workersWaitGroup.Wait()
}
