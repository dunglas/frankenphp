//go:build newworker

package frankenphp

// #cgo CFLAGS: -DNEW_WORKER
// #include <stdlib.h>
// #include "frankenphp.h"
import "C"
import "net/http"

type worker struct{}

func (worker *worker) handleRequest(r *http.Request) {}

var workers = make(map[string]*worker)

func initWorkers(opt []workerOpt) error {
	panic("not implemented")
}

func stopWorkers() {}

func drainWorkers() {}

func restartWorkers(workerOpts []workerOpt) {}

//export go_frankenphp_worker_handle_request_start
func go_frankenphp_worker_handle_request_start(threadIndex C.uintptr_t) C.bool {
	panic("not implemented")
}

//export go_frankenphp_finish_request
func go_frankenphp_finish_request(threadIndex C.uintptr_t, isWorkerRequest bool) {
}

func restartWorkersOnFileChanges(workerOpts []workerOpt) error {
	panic("not implemented")
}
