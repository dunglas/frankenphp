package frankenphp

// #include <stdlib.h>
// #include "frankenphp.h"
import "C"
import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"runtime/cgo"
	"sync"

	"go.uber.org/zap"
)

var (
	workersRequestChans sync.Map // map[fileName]chan *http.Request
	workersReadyWG      sync.WaitGroup
	workersWG           sync.WaitGroup
)

func startWorkers(fileName string, nbWorkers int) error {
	absFileName, err := filepath.Abs(fileName)
	if err != nil {
		return fmt.Errorf("workers %q: %w", fileName, err)
	}

	if _, ok := workersRequestChans.Load(absFileName); ok {
		return fmt.Errorf("workers %q: already started", absFileName)
	}

	workersRequestChans.Store(absFileName, make(chan *http.Request))
	shutdownWG.Add(nbWorkers)
	workersReadyWG.Add(nbWorkers)

	var (
		m      sync.Mutex
		errors []error
	)

	l := getLogger()
	for i := 0; i < nbWorkers; i++ {
		go func() {
			defer shutdownWG.Done()
			for {
				// Create main dummy request
				fc := &FrankenPHPContext{
					Env: map[string]string{"SCRIPT_FILENAME": absFileName},
				}
				r, err := http.NewRequestWithContext(context.WithValue(
					context.Background(),
					contextKey,
					fc,
				), "GET", "", nil)

				if err != nil {
					// TODO: this should never happen, maybe can we remove this block?
					m.Lock()
					defer m.Unlock()
					errors = append(errors, fmt.Errorf("workers %q: unable to create main worker request: %w", absFileName, err))

					return
				}

				l.Debug("starting", zap.String("worker", absFileName))
				if err := ServeHTTP(nil, r); err != nil {
					m.Lock()
					defer m.Unlock()
					errors = append(errors, fmt.Errorf("workers %q: unable to start: %w", absFileName, err))

					return
				}

				if fc.currentWorkerRequest != 0 {
					// Terminate the pending HTTP request handled by the worker
					maybeCloseContext(fc.currentWorkerRequest.Value().(*http.Request).Context().Value(contextKey).(*FrankenPHPContext))
					fc.currentWorkerRequest.Delete()
					fc.currentWorkerRequest = 0
				}

				// TODO: make the max restart configurable
				if _, ok := workersRequestChans.Load(absFileName); ok {
					workersReadyWG.Add(1)
					l.Error("unexpected termination, restarting", zap.String("worker", absFileName))
				} else {
					break
				}
			}

			// TODO: check if the termination is expected
			l.Debug("terminated", zap.String("worker", absFileName))
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
		workersRequestChans.Delete(k)
		close(v.(chan *http.Request))

		return true
	})
}

//export go_frankenphp_worker_ready
func go_frankenphp_worker_ready() {
	workersReadyWG.Done()
}

//export go_frankenphp_worker_handle_request_start
func go_frankenphp_worker_handle_request_start(mrh C.uintptr_t) C.uintptr_t {
	mainRequest := cgo.Handle(mrh).Value().(*http.Request)
	fc := mainRequest.Context().Value(contextKey).(*FrankenPHPContext)

	v, ok := workersRequestChans.Load(fc.Env["SCRIPT_FILENAME"])
	if !ok {
		// Probably shutting down
		return 0
	}

	rc := v.(chan *http.Request)

	l := getLogger()

	l.Debug("waiting for request", zap.String("worker", fc.Env["SCRIPT_FILENAME"]))
	r, ok := <-rc
	if !ok {
		// channel closed, server is shutting down
		l.Debug("shutting down", zap.String("worker", fc.Env["SCRIPT_FILENAME"]))

		return 0
	}

	fc.currentWorkerRequest = cgo.NewHandle(r)

	l.Debug("request handling started", zap.String("worker", fc.Env["SCRIPT_FILENAME"]), zap.String("url", r.RequestURI))
	if err := updateServerContext(r, false, mrh); err != nil {
		// Unexpected error
		l.Debug("unexpected error", zap.String("worker", fc.Env["SCRIPT_FILENAME"]), zap.String("url", r.RequestURI), zap.Error(err))

		return 0
	}

	return C.uintptr_t(fc.currentWorkerRequest)
}

//export go_frankenphp_finish_request
func go_frankenphp_finish_request(mrh, rh C.uintptr_t, deleteHandle bool) {
	rHandle := cgo.Handle(rh)
	r := rHandle.Value().(*http.Request)
	fc := r.Context().Value(contextKey).(*FrankenPHPContext)

	if deleteHandle {
		rHandle.Delete()

		cgo.Handle(mrh).Value().(*http.Request).Context().Value(contextKey).(*FrankenPHPContext).currentWorkerRequest = 0
	}

	maybeCloseContext(fc)

	var fields []zap.Field
	if mrh == 0 {
		fields = append(fields, zap.String("worker", fc.Env["SCRIPT_FILENAME"]), zap.String("url", r.RequestURI))
	} else {
		fields = append(fields, zap.String("url", r.RequestURI))
	}

	fc.Logger.Debug("request handling finished", fields...)
}
