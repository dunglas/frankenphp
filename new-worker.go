//go:build newworker

package frankenphp

// #cgo CFLAGS: -DNEW_WORKER
// #include <stdlib.h>
// #include "frankenphp.h"
import "C"
import "net/http"

// worker represents a PHP worker script.
type worker struct{}

// handleRequest handles an incoming HTTP request and passes it to the worker thread.
func (worker *worker) handleRequest(r *http.Request) {}

// A map of workers by identity.
var workers = make(map[string]*worker)

// initWorkers initializes the workers.
func initWorkers(opt []workerOpt) error {
	panic("not implemented")
}

// stopWorkers stops the workers.
func stopWorkers() {}

// drainWorkers drains the workers.
func drainWorkers() {}

// restartWorkers restarts the workers.
func restartWorkers(workerOpts []workerOpt) {}

// go_frankenphp_worker_handle_request_start handles the start of a worker request.
//
//export go_frankenphp_worker_handle_request_start
func go_frankenphp_worker_handle_request_start(threadIndex C.uintptr_t) C.bool {
	panic("not implemented")
}

// go_frankenphp_finish_request should flush the buffers and return the response.
// this does not mean the php code has finished executing,
// but that the request has been fully processed and can be returned to the client.
//
//export go_frankenphp_finish_request
func go_frankenphp_finish_request(threadIndex C.uintptr_t, isWorkerRequest bool) {
}

// restartWorkersOnFileChanges restarts the workers on file changes.
func restartWorkersOnFileChanges(workerOpts []workerOpt) error {
	panic("not implemented")
}
