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
	w := &worker{
		fileName:    absFileName,
		num:         o.num,
		env:         o.env,
		requestChan: make(chan *http.Request),
	}
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
	ready := sync.WaitGroup{}
	for _, worker := range workers {
		worker.threadMutex.RLock()
		ready.Add(len(worker.threads))
		for _, thread := range worker.threads {
			thread.state.set(stateRestarting)
			close(thread.drainChan)
			go func(thread *phpThread) {
				thread.state.waitFor(stateYielding)
				ready.Done()
			}(thread)
		}
	}
	stopWorkers()
	ready.Wait()
	for _, worker := range workers {
		for _, thread := range worker.threads {
			thread.drainChan = make(chan struct{})
			thread.state.set(stateReady)
		}
		worker.threadMutex.RUnlock()
	}
	workersDone = make(chan interface{})
}

func getDirectoriesToWatch(workerOpts []workerOpt) []string {
	directoriesToWatch := []string{}
	for _, w := range workerOpts {
		directoriesToWatch = append(directoriesToWatch, w.watch...)
	}
	return directoriesToWatch
}

func (worker *worker) startNewThread() {
	thread := getInactivePHPThread()
	convertToWorkerThread(thread, worker)
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
