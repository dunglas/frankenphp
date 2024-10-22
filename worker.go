package frankenphp

// #include <stdlib.h>
// #include "frankenphp.h"
import "C"
import (
	"fmt"
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
}

const maxWorkerErrorBackoff = 1 * time.Second
const minWorkerErrorBackoff = 100 * time.Millisecond
const maxWorkerConsecutiveFailures = 6

var (
	watcherIsEnabled bool
	workersReadyWG   sync.WaitGroup
	workerShutdownWG sync.WaitGroup
	workersAreReady  atomic.Bool
	workersAreDone   atomic.Bool
	workersDone      chan interface{}
	workers          map[string]*worker = make(map[string]*worker)
)

func initWorkers(opt []workerOpt) error {
	workersDone = make(chan interface{})
	workersAreReady.Store(false)
	workersAreDone.Store(false)

	for _, o := range opt {
		worker, err := newWorker(o)
		if err != nil {
			return err
		}
		workersReadyWG.Add(worker.num)
		for i := 0; i < worker.num; i++ {
			go worker.startNewWorkerThread()
		}
	}

	workersReadyWG.Wait()
	workersAreReady.Store(true)

	return nil
}

func newWorker(o workerOpt) (*worker, error) {
	absFileName, err := filepath.Abs(o.fileName)
	if err != nil {
		return nil, fmt.Errorf("worker filename is invalid %q: %w", o.fileName, err)
	}

	// if the worker already exists, return it
	// it's necessary since we don't want to destroy the channels when restarting on file changes
	if w, ok := workers[absFileName]; ok {
		return w, nil
	}

	if o.env == nil {
		o.env = make(PreparedEnv, 1)
	}

	o.env["FRANKENPHP_WORKER\x00"] = "1"
	w := &worker{fileName: absFileName, num: o.num, env: o.env, requestChan: make(chan *http.Request)}
	workers[absFileName] = w

	return w, nil
}

func (worker *worker) startNewWorkerThread() {
	workerShutdownWG.Add(1)
	defer workerShutdownWG.Done()

	backoff := minWorkerErrorBackoff
	failureCount := 0
	backingOffLock := sync.RWMutex{}

	for {

		// if the worker can stay up longer than backoff*2, it is probably an application error
		upFunc := sync.Once{}
		go func() {
			backingOffLock.RLock()
			wait := backoff * 2
			backingOffLock.RUnlock()
			time.Sleep(wait)
			upFunc.Do(func() {
				backingOffLock.Lock()
				defer backingOffLock.Unlock()
				// if we come back to a stable state, reset the failure count
				if backoff == minWorkerErrorBackoff {
					failureCount = 0
				}

				// earn back the backoff over time
				if failureCount > 0 {
					backoff = max(backoff/2, 100*time.Millisecond)
				}
			})
		}()

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
		if workersAreDone.Load() {
			break
		}

		// on exit status 0 we just run the worker script again
		if fc.exitStatus == 0 {
			// TODO: make the max restart configurable
			if c := logger.Check(zapcore.InfoLevel, "restarting"); c != nil {
				c.Write(zap.String("worker", worker.fileName))
			}
			metrics.StopWorker(worker.fileName, StopReasonRestart)
			continue
		}

		// on exit status 1 we log the error and apply an exponential backoff when restarting
		upFunc.Do(func() {
			backingOffLock.Lock()
			defer backingOffLock.Unlock()
			// if we end up here, the worker has not been up for backoff*2
			// this is probably due to a syntax error or another fatal error
			if failureCount >= maxWorkerConsecutiveFailures {
				if !watcherIsEnabled {
					panic(fmt.Errorf("workers %q: too many consecutive failures", worker.fileName))
				}
				logger.Warn("many consecutive worker failures", zap.String("worker", worker.fileName), zap.Int("failures", failureCount))
			}
			failureCount += 1
		})
		backingOffLock.RLock()
		wait := backoff
		backingOffLock.RUnlock()
		time.Sleep(wait)
		backingOffLock.Lock()
		backoff *= 2
		backoff = min(backoff, maxWorkerErrorBackoff)
		backingOffLock.Unlock()
		metrics.StopWorker(worker.fileName, StopReasonCrash)
	}

	metrics.StopWorker(worker.fileName, StopReasonShutdown)

	// TODO: check if the termination is expected
	if c := logger.Check(zapcore.DebugLevel, "terminated"); c != nil {
		c.Write(zap.String("worker", worker.fileName))
	}
}

func stopWorkers() {
	workersAreDone.Store(true)
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
	directoriesToWatch := []string{}
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
	metrics.ReadyWorker(fc.scriptFilename)
	worker, ok := workers[fc.scriptFilename]
	if !ok {
		panic("worker not found for script: " + fc.scriptFilename)
	}
	thread.worker = worker
	if !workersAreReady.Load() {
		workersReadyWG.Done()
	}
	// TODO: we can also store all threads assigned to the worker if needed
}

//export go_frankenphp_worker_handle_request_start
func go_frankenphp_worker_handle_request_start(threadIndex C.uintptr_t) C.bool {
	thread := phpThreads[threadIndex]

	// we assign a worker to the thread if it doesn't have one already
	if thread.worker == nil {
		assignThreadToWorker(thread)
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
		executePHPFunction("opcache_reset")

		return C.bool(false)
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

		return C.bool(false)
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

	thread.Unpin()
}
