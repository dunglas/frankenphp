package frankenphp

// #include <stdlib.h>
// #include "frankenphp.h"
import "C"
import (
	"context"
	"fmt"
	"log"
	"net/http"
	"runtime/cgo"
	"sync"
)

var (
	workersRequestChans sync.Map // map[fileName]chan *http.Request
	workersReadyWG      sync.WaitGroup
	workersWG           sync.WaitGroup
)

func startWorkers(fileName string, nbWorkers int) error {
	if _, ok := workersRequestChans.Load(fileName); ok {
		panic(fmt.Errorf("workers %q: already started", fileName))
	}

	workersRequestChans.Store(fileName, make(chan *http.Request))
	shutdownWG.Add(nbWorkers)
	workersReadyWG.Add(nbWorkers)

	var (
		m      sync.Mutex
		errors []error
	)
	for i := 0; i < nbWorkers; i++ {
		go func() {
			defer shutdownWG.Done()

			// Create main dummy request
			r, err := http.NewRequest("GET", "", nil)
			if err != nil {
				m.Lock()
				defer m.Unlock()
				errors = append(errors, fmt.Errorf("workers %q: unable to create main worker request: %w", fileName, err))

				return
			}

			ctx := context.WithValue(
				r.Context(),
				contextKey,
				&FrankenPHPContext{
					Env: map[string]string{"SCRIPT_FILENAME": fileName},
				},
			)

			log.Printf("worker %q: starting", fileName)
			if err := ServeHTTP(nil, r.WithContext(ctx)); err != nil {
				m.Lock()
				defer m.Unlock()
				errors = append(errors, fmt.Errorf("workers %q: unable to start: %w", fileName, err))

				return
			}
			log.Printf("worker %q: terminated", fileName)
		}()
	}

	workersReadyWG.Wait()
	m.Lock()
	defer m.Unlock()

	if len(errors) == 0 {
		return nil
	}

	// Wrapping multiple errors will be available in Go 1.20: https://github.com/golang/go/issues/53435
	return fmt.Errorf("workers %q: error while starting: #%v", fileName, errors)
}

func stopWorkers() {
	workersRequestChans.Range(func(k, v any) bool {
		close(v.(chan *http.Request))
		workersRequestChans.Delete(k)

		return true
	})
}

//export go_frankenphp_worker_ready
func go_frankenphp_worker_ready() {
	workersReadyWG.Done()
}

//export go_frankenphp_worker_handle_request_start
func go_frankenphp_worker_handle_request_start(rh C.uintptr_t) C.uintptr_t {
	previousRequest := cgo.Handle(rh).Value().(*http.Request)
	previousFc := previousRequest.Context().Value(contextKey).(*FrankenPHPContext)

	v, ok := workersRequestChans.Load(previousFc.Env["SCRIPT_FILENAME"])
	if !ok {
		// Probably shutting down
		return 0
	}

	rc := v.(chan *http.Request)

	log.Printf("worker %q: waiting for request", previousFc.Env["SCRIPT_FILENAME"])
	r, ok := <-rc
	if !ok {
		// channel closed, server is shutting down
		log.Printf("worker %q: shutting down", previousFc.Env["SCRIPT_FILENAME"])

		return 0
	}

	log.Printf("worker %q: handling request %#v", previousFc.Env["SCRIPT_FILENAME"], r)
	if err := updateServerContext(r); err != nil {
		// Unexpected error
		log.Printf("worker %q: unexpected error: %s", previousFc.Env["SCRIPT_FILENAME"], err)

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

	log.Printf("worker %q: finished handling request %#v", fc.Env["SCRIPT_FILENAME"], r)
}
