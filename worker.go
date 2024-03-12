package frankenphp

// #include <stdlib.h>
// #include "frankenphp.h"
import "C"
import (
	"errors"
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
)

// TODO: start all the worker in parallell to reduce the boot time
func initWorkers(opt []workerOpt) error {
	for _, w := range opt {
		if err := startWorkers(w.fileName, w.num, w.env); err != nil {
			return err
		}
	}

	return nil
}

func startWorkers(fileName string, nbWorkers int, env PreparedEnv) error {
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
		m    sync.RWMutex
		errs []error
	)

	if env == nil {
		env = make(PreparedEnv, 1)
	}

	env["FRANKENPHP_WORKER\x00"] = "1\x00"

	l := getLogger()
	for i := 0; i < nbWorkers; i++ {
		go func() {
			defer shutdownWG.Done()
			for {
				// Create main dummy request
				r, err := http.NewRequest(http.MethodGet, filepath.Base(absFileName), nil)
				if err != nil {
					panic(err)
				}
				r, err = NewRequestWithContext(
					r,
					WithRequestDocumentRoot(filepath.Dir(absFileName), false),
					WithRequestPreparedEnv(env),
				)
				if err != nil {
					panic(err)
				}

				l.Debug("starting", zap.String("worker", absFileName))
				if err := ServeHTTP(nil, r); err != nil {
					panic(err)
				}

				fc := r.Context().Value(contextKey).(*FrankenPHPContext)
				if fc.currentWorkerRequest != 0 {
					// Terminate the pending HTTP request handled by the worker
					maybeCloseContext(fc.currentWorkerRequest.Value().(*http.Request).Context().Value(contextKey).(*FrankenPHPContext))
					fc.currentWorkerRequest.Delete()
					fc.currentWorkerRequest = 0
				}

				// TODO: make the max restart configurable
				if _, ok := workersRequestChans.Load(absFileName); ok {
					workersReadyWG.Add(1)
					if fc.exitStatus == 0 {
						l.Info("restarting", zap.String("worker", absFileName))
					} else {
						l.Error("unexpected termination, restarting", zap.String("worker", absFileName), zap.Int("exit_status", int(fc.exitStatus)))
					}
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

	if len(errs) == 0 {
		return nil
	}

	return fmt.Errorf("workers %q: error while starting: %w", fileName, errors.Join(errs...))
}

func stopWorkers() {
	workersRequestChans.Range(func(k, v any) bool {
		workersRequestChans.Delete(k)

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

	v, ok := workersRequestChans.Load(fc.scriptFilename)
	if !ok {
		// Probably shutting down
		return 0
	}

	rc := v.(chan *http.Request)

	l := getLogger()

	l.Debug("waiting for request", zap.String("worker", fc.scriptFilename))

	var r *http.Request
	select {
	case <-done:
		l.Debug("shutting down", zap.String("worker", fc.scriptFilename))

		return 0
	case r = <-rc:
	}

	fc.currentWorkerRequest = cgo.NewHandle(r)
	r.Context().Value(handleKey).(*handleList).AddHandle(fc.currentWorkerRequest)

	l.Debug("request handling started", zap.String("worker", fc.scriptFilename), zap.String("url", r.RequestURI))
	if err := updateServerContext(r, false, mrh); err != nil {
		// Unexpected error
		l.Debug("unexpected error", zap.String("worker", fc.scriptFilename), zap.String("url", r.RequestURI), zap.Error(err))

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
		r.Context().Value(handleKey).(*handleList).FreeAll()
		cgo.Handle(mrh).Value().(*http.Request).Context().Value(contextKey).(*FrankenPHPContext).currentWorkerRequest = 0
	}

	maybeCloseContext(fc)

	var fields []zap.Field
	if mrh == 0 {
		fields = append(fields, zap.String("worker", fc.scriptFilename), zap.String("url", r.RequestURI))
	} else {
		fields = append(fields, zap.String("url", r.RequestURI))
	}

	fc.logger.Debug("request handling finished", fields...)
}
