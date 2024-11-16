package frankenphp

// #include "frankenphp.h"
import "C"
import (
	"fmt"
	"github.com/dunglas/frankenphp/internal/fastabs"
	"net/http"
	"path/filepath"
	"sync"
	"sync/atomic"
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
	workers              map[string]*worker
	workersDone          chan interface{}
	watcherIsEnabled     bool
	workersAreRestarting atomic.Bool
	workerRestartWG      sync.WaitGroup
	workerShutdownWG     sync.WaitGroup
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
	workerRestartWG.Add(1)
	defer workerRestartWG.Done()
	for _, worker := range workers {
		workerShutdownWG.Add(worker.num)
	}
	workersAreRestarting.Store(true)
	stopWorkers()
	workerShutdownWG.Wait()
	workersDone = make(chan interface{})
	workersAreRestarting.Store(false)
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

func (worker *worker) beforeScript(thread *phpThread) {
	// if we are restarting due to file watching, wait for all workers to finish first
	if workersAreRestarting.Load() {
		workerShutdownWG.Done()
		workerRestartWG.Wait()
	}

	thread.backoff.reset()
	metrics.StartWorker(worker.fileName)

	// Create a dummy request to set up the worker
	r, err := http.NewRequest(http.MethodGet, filepath.Base(worker.fileName), nil)
	if err != nil {
		panic(err)
	}

	r, err = NewRequestWithContext(
		r,
		WithRequestDocumentRoot(filepath.Dir(worker.fileName), false),
		WithRequestPreparedEnv(worker.env),
	)
	if err != nil {
		panic(err)
	}

	if err := updateServerContext(thread, r, true, false); err != nil {
		panic(err)
	}

	thread.mainRequest = r
	if c := logger.Check(zapcore.DebugLevel, "starting"); c != nil {
		c.Write(zap.String("worker", worker.fileName), zap.Int("thread", thread.threadIndex))
	}
}

func (worker *worker) afterScript(thread *phpThread, exitStatus int) {
	fc := thread.mainRequest.Context().Value(contextKey).(*FrankenPHPContext)
	fc.exitStatus = exitStatus

	defer func() {
		maybeCloseContext(fc)
		thread.mainRequest = nil
	}()

	// on exit status 0 we just run the worker script again
	if fc.exitStatus == 0 {
		// TODO: make the max restart configurable
		metrics.StopWorker(worker.fileName, StopReasonRestart)

		if c := logger.Check(zapcore.DebugLevel, "restarting"); c != nil {
			c.Write(zap.String("worker", worker.fileName))
		}
		return
	}

	// on exit status 1 we apply an exponential backoff when restarting
	metrics.StopWorker(worker.fileName, StopReasonCrash)
	thread.backoff.trigger(func(failureCount int) {
		// if we end up here, the worker has not been up for backoff*2
		// this is probably due to a syntax error or another fatal error
		if !watcherIsEnabled {
			panic(fmt.Errorf("workers %q: too many consecutive failures", worker.fileName))
		}
		logger.Warn("many consecutive worker failures", zap.String("worker", worker.fileName), zap.Int("failures", failureCount))
	})
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
	thread := phpThreads[threadIndex]

	if c := logger.Check(zapcore.DebugLevel, "waiting for request"); c != nil {
		c.Write(zap.String("worker", thread.worker.fileName))
	}

	var r *http.Request
	select {
	case <-workersDone:
		if c := logger.Check(zapcore.DebugLevel, "shutting down"); c != nil {
			c.Write(zap.String("worker", thread.worker.fileName))
		}

		// execute opcache_reset if the restart was triggered by the watcher
		if watcherIsEnabled && workersAreRestarting.Load() && !executePHPFunction("opcache_reset") {
			logger.Error("failed to call opcache_reset")
		}

		return C.bool(false)
	case r = <-thread.requestChan:
	case r = <-thread.worker.requestChan:
	}

	thread.workerRequest = r

	if c := logger.Check(zapcore.DebugLevel, "request handling started"); c != nil {
		c.Write(zap.String("worker", thread.worker.fileName), zap.String("url", r.RequestURI))
	}

	if err := updateServerContext(thread, r, false, true); err != nil {
		// Unexpected error
		if c := logger.Check(zapcore.DebugLevel, "unexpected error"); c != nil {
			c.Write(zap.String("worker", thread.worker.fileName), zap.String("url", r.RequestURI), zap.Error(err))
		}
		fc := r.Context().Value(contextKey).(*FrankenPHPContext)
		rejectRequest(fc.responseWriter, err.Error())
		maybeCloseContext(fc)
		thread.workerRequest = nil
		thread.Unpin()

		return go_frankenphp_worker_handle_request_start(threadIndex)
	}
	return C.bool(true)
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
