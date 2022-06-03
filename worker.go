package frankenphp

// #include <stdlib.h>
// #include "frankenphp.h"
import "C"
import (
	"fmt"
	"log"
	"net/http"
	"runtime"
	"runtime/cgo"
	"sync"
	"unsafe"
)

var requestsChans sync.Map // map[fileName]cgo.NewHandle(chan *http.Request)
var workersWaitGroup sync.WaitGroup

func WorkerHandleRequest(responseWriter http.ResponseWriter, request *http.Request) error {
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

func newWorker(fileName string, requestsChanHandle cgo.Handle) {
	go func() {
		workersWaitGroup.Add(1)
		runtime.LockOSThread()

		cFileName := C.CString(fileName)
		defer C.free(unsafe.Pointer(cFileName))

		if C.frankenphp_create_server_context(C.uintptr_t(requestsChanHandle), cFileName) < 0 {
			panic(fmt.Errorf("error during request context creation"))
		}

		if C.frankenphp_request_startup() < 0 {
			panic("error during PHP request startup")
		}

		log.Printf("worker %q: started", fileName)

		if C.frankenphp_execute_script(cFileName) < 0 {
			panic("error during PHP script execution")
		}

		C.frankenphp_request_shutdown()

		log.Printf("worker %q: shutting down", fileName)
		workersWaitGroup.Done()
	}()
}

//export go_frankenphp_worker_handle_request_start
func go_frankenphp_worker_handle_request_start(rch C.uintptr_t) C.uintptr_t {
	rc := cgo.Handle(rch).Value().(chan *http.Request)

	log.Print("worker: waiting for request")
	r, ok := <-rc
	if !ok {
		// channel closed, server is shutting down
		log.Print("worker: breaking from the main loop")

		return 0
	}

	log.Printf("worker: handling request %#v", r)

	fc := r.Context().Value(contextKey).(*FrankenPHPContext)
	if fc == nil || fc.responseWriter == nil {
		panic("worker: not a valid worker request")
	}

	if err := updateServerContext(r); err != nil {
		// Unexpected error
		log.Print(err)

		return 0
	}

	return C.uintptr_t(cgo.NewHandle(r))
}

//export go_frankenphp_worker_handle_request_end
func go_frankenphp_worker_handle_request_end(rh C.uintptr_t) {
	rHandle := cgo.Handle(rh)
	r := rHandle.Value().(*http.Request)
	fc := r.Context().Value(contextKey).(*FrankenPHPContext)

	cgo.Handle(rh).Delete()

	close(fc.done)
}
