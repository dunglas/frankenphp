package frankenphp

// #include "frankenphp.h"
import "C"
import (
	"fmt"
	"github.com/dunglas/frankenphp/internal/fastabs"
	"net/http"
	"path/filepath"
	"sync"
	"time"

	"github.com/dunglas/frankenphp/internal/watcher"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type worker struct {
	fileName    string
	num         int
	env         PreparedEnv
	requestChan chan *http.Request
	threads     []*phpThread
	threadMutex sync.RWMutex
}


var (
	workers          map[string]*worker
	workersDone      chan interface{}
	watcherIsEnabled bool
)

func initWorkers(opt []workerOpt) error {
	workers = make(map[string]*worker, len(opt))
	workersDone = make(chan interface{})
	directoriesToWatch := getDirectoriesToWatch(opt)
	watcherIsEnabled = len(directoriesToWatch) > 0

	for _, o := range opt {
		worker, err := newWorker(o)
		worker.threads = make([]*phpThread, 0, o.num)
		if err != nil {
			return err
		}
		for i := 0; i < worker.num; i++ {
			worker.startNewThread()
		}
	}

	if len(directoriesToWatch) == 0 {
		return nil
	}

	if err := watcher.InitWatcher(directoriesToWatch, restartWorkers, getLogger()); err != nil {
		return err
	}

	return nil
}

func newWorker(o workerOpt) (*worker, error) {
	absFileName, err := fastabs.FastAbs(o.fileName)
	if err != nil {
		return nil, fmt.Errorf("worker filename is invalid %q: %w", o.fileName, err)
	}

	if o.env == nil {
		o.env = make(PreparedEnv, 1)
	}

	o.env["FRANKENPHP_WORKER\x00"] = "1"
	w := &worker{fileName: absFileName, num: o.num, env: o.env, requestChan: make(chan *http.Request)}
	workers[absFileName] = w

	return w, nil
}

func stopWorkers() {
	close(workersDone)
}

func drainWorkers() {
	watcher.DrainWatcher()
	stopWorkers()
}

func restartWorkers() {
	restart := sync.WaitGroup{}
	restart.Add(1)
	ready := sync.WaitGroup{}
	for _, worker := range workers {
		worker.threadMutex.RLock()
		ready.Add(len(worker.threads))
		for _, thread := range worker.threads {
			thread.state.set(stateRestarting)
			go func(thread *phpThread) {
				thread.state.waitForAndYield(&restart, stateReady)
				ready.Done()
			}(thread)
		}
		worker.threadMutex.RUnlock()
	}
	stopWorkers()
	ready.Wait()
	workersDone = make(chan interface{})
	restart.Done()
}

func getDirectoriesToWatch(workerOpts []workerOpt) []string {
	directoriesToWatch := []string{}
	for _, w := range workerOpts {
		directoriesToWatch = append(directoriesToWatch, w.watch...)
	}
	return directoriesToWatch
}

func (worker *worker) startNewThread() {
	getInactivePHPThread().setActive(
		// onStartup => right before the thread is ready
		func(thread *phpThread) {
			thread.worker = worker
			thread.scriptName = worker.fileName
			thread.requestChan = make(chan *http.Request)
			thread.backoff = newExponentialBackoff()
			worker.threadMutex.Lock()
			worker.threads = append(worker.threads, thread)
			worker.threadMutex.Unlock()
			metrics.ReadyWorker(worker.fileName)
		},
		// beforeScriptExecution => set up the worker with a fake request
		func(thread *phpThread) {
			worker.beforeScript(thread)
		},
		// afterScriptExecution => tear down the worker
		func(thread *phpThread, exitStatus int) {
			worker.afterScript(thread, exitStatus)
		},
		// onShutdown => after the thread is done
		func(thread *phpThread) {
			thread.worker = nil
			thread.backoff = nil
		},
	)
}

func (worker *worker) handleRequest(r *http.Request, fc *FrankenPHPContext) {
	metrics.StartWorkerRequest(fc.scriptFilename)

	// dispatch requests to all worker threads in order
	worker.threadMutex.RLock()
	for _, thread := range worker.threads {
		select {
		case thread.requestChan <- r:
			worker.threadMutex.RUnlock()
			<-fc.done
			metrics.StopWorkerRequest(worker.fileName, time.Since(fc.startedAt))
			return
		default:
		}
	}
	worker.threadMutex.RUnlock()

	// if no thread was available, fan the request out to all threads
	// TODO: theoretically there could be autoscaling of threads here
	worker.requestChan <- r
	<-fc.done
	metrics.StopWorkerRequest(worker.fileName, time.Since(fc.startedAt))
}

//export go_frankenphp_worker_handle_request_start
func go_frankenphp_worker_handle_request_start(threadIndex C.uintptr_t) C.bool {
	thread := phpWorkerThread(phpThreads[threadIndex])
	return C.bool(thread.stateMachine.waitForWorkerRequest(stateReady))
}

//export go_frankenphp_finish_worker_request
func go_frankenphp_finish_worker_request(threadIndex C.uintptr_t) {
	thread := phpThreads[threadIndex]
	r := thread.getActiveRequest()
	fc := r.Context().Value(contextKey).(*FrankenPHPContext)

	maybeCloseContext(fc)
	thread.workerRequest = nil
	thread.Unpin()

	if c := fc.logger.Check(zapcore.DebugLevel, "request handling finished"); c != nil {
		c.Write(zap.String("worker", fc.scriptFilename), zap.String("url", r.RequestURI))
	}
}

// when frankenphp_finish_request() is directly called from PHP
//
//export go_frankenphp_finish_php_request
func go_frankenphp_finish_php_request(threadIndex C.uintptr_t) {
	r := phpThreads[threadIndex].getActiveRequest()
	fc := r.Context().Value(contextKey).(*FrankenPHPContext)
	maybeCloseContext(fc)

	if c := fc.logger.Check(zapcore.DebugLevel, "request handling finished"); c != nil {
		c.Write(zap.String("url", r.RequestURI))
	}
}
