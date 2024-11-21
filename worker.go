package frankenphp

// #include <stdlib.h>
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
	ready       chan struct{}
}

var (
	watcherIsEnabled bool
	workerShutdownWG sync.WaitGroup
	workersDone      chan interface{}
	workers          = make(map[string]*worker)
)

func initWorkers(opt []workerOpt) error {
	workersDone = make(chan interface{})

	ready := sync.WaitGroup{}

	for _, o := range opt {
		worker, err := newWorker(o)
		worker.threads = make([]*phpThread, 0, o.num)
		if err != nil {
			return err
		}
		for i := 0; i < worker.num; i++ {
			go worker.startNewWorkerThread()
		}
		ready.Add(1)
		go func() {
			for i := 0; i < worker.num; i++ {
				<-worker.ready
			}
			ready.Done()
		}()
	}

	ready.Wait()

	return nil
}

func newWorker(o workerOpt) (*worker, error) {
	absFileName, err := fastabs.FastAbs(o.fileName)
	if err != nil {
		return nil, fmt.Errorf("worker filename is invalid %q: %w", o.fileName, err)
	}

	// if the worker already exists, return it,
	// it's necessary since we don't want to destroy the channels when restarting on file changes
	if w, ok := workers[absFileName]; ok {
		return w, nil
	}

	if o.env == nil {
		o.env = make(PreparedEnv, 1)
	}

	o.env["FRANKENPHP_WORKER\x00"] = "1"
	w := &worker{
		fileName:    absFileName,
		num:         o.num,
		env:         o.env,
		requestChan: make(chan *http.Request),
		ready:       make(chan struct{}, o.num),
	}
	workers[absFileName] = w

	return w, nil
}

func (worker *worker) startNewWorkerThread() {
	workerShutdownWG.Add(1)
	defer workerShutdownWG.Done()
	backoff := &exponentialBackoff{
		maxBackoff:             1 * time.Second,
		minBackoff:             100 * time.Millisecond,
		maxConsecutiveFailures: 6,
	}

	for {
		// if the worker can stay up longer than backoff*2, it is probably an application error
		backoff.wait()

		metrics.StartWorker(worker.fileName)

		// Create main dummy request
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

		if c := logger.Check(zapcore.DebugLevel, "starting"); c != nil {
			c.Write(zap.String("worker", worker.fileName), zap.Int("num", worker.num))
		}

		if err := ServeHTTP(nil, r); err != nil {
			panic(err)
		}

		fc := r.Context().Value(contextKey).(*FrankenPHPContext)

		// if we are done, exit the loop that restarts the worker script
		select {
		case _, ok := <-workersDone:
			if !ok {
				metrics.StopWorker(worker.fileName, StopReasonShutdown)

				if c := logger.Check(zapcore.DebugLevel, "terminated"); c != nil {
					c.Write(zap.String("worker", worker.fileName))
				}

				return
			}
			// continue on since the channel is still open
		default:
			// continue on since the channel is still open
		}

		// on exit status 0 we just run the worker script again
		if fc.exitStatus == 0 {
			// TODO: make the max restart configurable
			if c := logger.Check(zapcore.InfoLevel, "restarting"); c != nil {
				c.Write(zap.String("worker", worker.fileName))
			}
			metrics.StopWorker(worker.fileName, StopReasonRestart)
			backoff.recordSuccess()
			continue
		}

		// on exit status 1 we log the error and apply an exponential backoff when restarting
		if backoff.recordFailure() {
			if !watcherIsEnabled {
				panic(fmt.Errorf("workers %q: too many consecutive failures", worker.fileName))
			}
			logger.Warn("many consecutive worker failures", zap.String("worker", worker.fileName), zap.Int("failures", backoff.failureCount))
		}
		metrics.StopWorker(worker.fileName, StopReasonCrash)
	}

	// unreachable
}

func (worker *worker) handleRequest(r *http.Request) {
	worker.threadMutex.RLock()
	// dispatch requests to all worker threads in order
	for _, thread := range worker.threads {
		select {
		case thread.requestChan <- r:
			worker.threadMutex.RUnlock()
			return
		default:
		}
	}
	worker.threadMutex.RUnlock()
	// if no thread was available, fan the request out to all threads
	// TODO: theoretically there could be autoscaling of threads here
	worker.requestChan <- r
}

func stopWorkers() {
	close(workersDone)
}

func drainWorkers() {
	watcher.DrainWatcher()
	watcherIsEnabled = false
	stopWorkers()
	workerShutdownWG.Wait()
	workers = make(map[string]*worker)
}

func restartWorkersOnFileChanges(workerOpts []workerOpt) error {
	var directoriesToWatch []string
	for _, w := range workerOpts {
		directoriesToWatch = append(directoriesToWatch, w.watch...)
	}
	watcherIsEnabled = len(directoriesToWatch) > 0
	if !watcherIsEnabled {
		return nil
	}
	restartWorkers := func() {
		restartWorkers(workerOpts)
	}
	if err := watcher.InitWatcher(directoriesToWatch, restartWorkers, getLogger()); err != nil {
		return err
	}

	return nil
}

func restartWorkers(workerOpts []workerOpt) {
	stopWorkers()
	workerShutdownWG.Wait()
	if err := initWorkers(workerOpts); err != nil {
		logger.Error("failed to restart workers when watching files")
		panic(err)
	}
	logger.Info("workers restarted successfully")
}

func assignThreadToWorker(thread *phpThread) {
	fc := thread.mainRequest.Context().Value(contextKey).(*FrankenPHPContext)
	worker, ok := workers[fc.scriptFilename]
	if !ok {
		panic("worker not found for script: " + fc.scriptFilename)
	}
	thread.worker = worker
	thread.requestChan = make(chan *http.Request)
	worker.threadMutex.Lock()
	worker.threads = append(worker.threads, thread)
	worker.threadMutex.Unlock()
}

//export go_frankenphp_worker_handle_request_start
func go_frankenphp_worker_handle_request_start(threadIndex C.uintptr_t) C.bool {
	thread := phpThreads[threadIndex]

	// we assign a worker to the thread if it doesn't have one already
	if thread.worker == nil {
		assignThreadToWorker(thread)
	}
	thread.readiedOnce.Do(func() {
		// inform metrics that the worker is ready
		metrics.ReadyWorker(thread.worker.fileName)
	})

	select {
	case thread.worker.ready <- struct{}{}:
	default:
	}

	if c := logger.Check(zapcore.DebugLevel, "waiting for request"); c != nil {
		c.Write(zap.String("worker", thread.worker.fileName))
	}

	var r *http.Request
	select {
	case <-workersDone:
		if c := logger.Check(zapcore.DebugLevel, "shutting down"); c != nil {
			c.Write(zap.String("worker", thread.worker.fileName))
		}
		thread.worker = nil
		C.frankenphp_reset_opcache()

		return C.bool(false)
	case r = <-thread.worker.requestChan:
	case r = <-thread.requestChan:
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

//export go_frankenphp_finish_request
func go_frankenphp_finish_request(threadIndex C.uintptr_t, isWorkerRequest bool) {
	thread := phpThreads[threadIndex]
	r := thread.getActiveRequest()
	fc := r.Context().Value(contextKey).(*FrankenPHPContext)

	if isWorkerRequest {
		thread.workerRequest = nil
	}

	maybeCloseContext(fc)

	if c := fc.logger.Check(zapcore.DebugLevel, "request handling finished"); c != nil {
		var fields []zap.Field
		if isWorkerRequest {
			fields = append(fields, zap.String("worker", fc.scriptFilename), zap.String("url", r.RequestURI))
		} else {
			fields = append(fields, zap.String("url", r.RequestURI))
		}

		c.Write(fields...)
	}

	if isWorkerRequest {
		thread.Unpin()
	}
}
