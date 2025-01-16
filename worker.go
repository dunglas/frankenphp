package frankenphp

// #include "frankenphp.h"
import "C"
import (
	"fmt"
	"github.com/dunglas/frankenphp/internal/fastabs"
	"net/http"
	"sync"
	"time"

	"github.com/dunglas/frankenphp/internal/watcher"
)

// represents a worker script and can have many threads assigned to it
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
	watcherIsEnabled bool
)

func initWorkers(opt []workerOpt) error {
	workers = make(map[string]*worker, len(opt))
	workersReady := sync.WaitGroup{}
	directoriesToWatch := getDirectoriesToWatch(opt)
	watcherIsEnabled = len(directoriesToWatch) > 0

	for _, o := range opt {
		worker, err := newWorker(o)
		worker.threads = make([]*phpThread, 0, o.num)
		workersReady.Add(o.num)
		if err != nil {
			return err
		}
		for i := 0; i < worker.num; i++ {
			thread := getInactivePHPThread()
			convertToWorkerThread(thread, worker)
			go func() {
				thread.state.waitFor(stateReady)
				workersReady.Done()
			}()
		}
	}

	workersReady.Wait()

	if !watcherIsEnabled {
		return nil
	}

	watcherIsEnabled = true
	if err := watcher.InitWatcher(directoriesToWatch, RestartWorkers, getLogger()); err != nil {
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
	w := &worker{
		fileName:    absFileName,
		num:         o.num,
		env:         o.env,
		requestChan: make(chan *http.Request),
	}
	workers[absFileName] = w

	return w, nil
}

func drainWorkers() {
	watcher.DrainWatcher()
}

// RestartWorkers attempts to restart all workers gracefully
func RestartWorkers() {
	// disallow scaling threads while restarting workers
	scalingMu.Lock()
	defer scalingMu.Unlock()

	ready := sync.WaitGroup{}
	threadsToRestart := make([]*phpThread, 0)
	for _, worker := range workers {
		worker.threadMutex.RLock()
		ready.Add(len(worker.threads))
		for _, thread := range worker.threads {
			if !thread.state.requestSafeStateChange(stateRestarting) {
				// no state change allowed == thread is shutting down
				// we'll proceed to restart all other threads anyways
				continue
			}
			close(thread.drainChan)
			threadsToRestart = append(threadsToRestart, thread)
			go func(thread *phpThread) {
				thread.state.waitFor(stateYielding)
				ready.Done()
			}(thread)
		}
		worker.threadMutex.RUnlock()
	}

	ready.Wait()

	for _, thread := range threadsToRestart {
		thread.drainChan = make(chan struct{})
		thread.state.set(stateReady)
	}
}

func getDirectoriesToWatch(workerOpts []workerOpt) []string {
	directoriesToWatch := []string{}
	for _, w := range workerOpts {
		directoriesToWatch = append(directoriesToWatch, w.watch...)
	}
	return directoriesToWatch
}

func (worker *worker) attachThread(thread *phpThread) {
	worker.threadMutex.Lock()
	worker.threads = append(worker.threads, thread)
	worker.threadMutex.Unlock()
}

func (worker *worker) detachThread(thread *phpThread) {
	worker.threadMutex.Lock()
	for i, t := range worker.threads {
		if t == thread {
			worker.threads = append(worker.threads[:i], worker.threads[i+1:]...)
			break
		}
	}
	worker.threadMutex.Unlock()
}

func (worker *worker) countThreads() int {
	worker.threadMutex.RLock()
	l := len(worker.threads)
	worker.threadMutex.RUnlock()

	return l
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
			// thread is busy, continue
		}
	}
	worker.threadMutex.RUnlock()

	// if no thread was available, mark the request as queued and apply the scaling strategy
	metrics.QueuedWorkerRequest(fc.scriptFilename)
	for {
		select {
		case worker.requestChan <- r:
			metrics.DequeuedWorkerRequest(fc.scriptFilename)
			<-fc.done
			metrics.StopWorkerRequest(worker.fileName, time.Since(fc.startedAt))
			return
		case scaleChan <- fc:
			// the request has triggered scaling, continue to wait for a thread
		}
	}
}
