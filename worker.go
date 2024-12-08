package frankenphp

// #include "frankenphp.h"
import "C"
import (
	"errors"
	"fmt"
	"github.com/dunglas/frankenphp/internal/fastabs"
	"net/http"
	"strings"
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
	directoriesToWatch := getDirectoriesToWatch(opt)
	watcherIsEnabled = len(directoriesToWatch) > 0

	for _, o := range opt {
		worker, err := newWorker(o)
		worker.threads = make([]*phpThread, 0, o.num)
		if err != nil {
			return err
		}
		for i := 0; i < worker.num; i++ {
			thread := getInactivePHPThread()
			convertToWorkerThread(thread, worker)
		}
	}

	if !watcherIsEnabled {
		return nil
	}

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

func AddWorkerThread(workerFileName string) (string, int, error) {
	worker := getWorkerByFilePattern(workerFileName)
	if worker == nil {
		return "", 0, errors.New("worker not found")
	}
	thread := getInactivePHPThread()
	if thread == nil {
		return "", 0, fmt.Errorf("max amount of threads reached: %d", len(phpThreads))
	}
	convertToWorkerThread(thread, worker)
	return worker.fileName, worker.countThreads(), nil
}

func RemoveWorkerThread(workerFileName string) (string, int, error) {
	worker := getWorkerByFilePattern(workerFileName)
	if worker == nil {
		return "", 0, errors.New("worker not found")
	}

	worker.threadMutex.RLock()
	if len(worker.threads) <= 1 {
		worker.threadMutex.RUnlock()
		return worker.fileName, 0, errors.New("cannot remove last thread")
	}
	thread := worker.threads[len(worker.threads)-1]
	worker.threadMutex.RUnlock()
	convertToInactiveThread(thread)

	return worker.fileName, worker.countThreads(), nil
}

// get the first worker ending in the given pattern
func getWorkerByFilePattern(pattern string) *worker {
	for _, worker := range workers {
		if pattern == "" || strings.HasSuffix(worker.fileName, pattern) {
			return worker
		}
	}

	return nil
}

func RestartWorkers() {
	ready := sync.WaitGroup{}
	for _, worker := range workers {
		worker.threadMutex.RLock()
		ready.Add(len(worker.threads))
		for _, thread := range worker.threads {
			// disallow changing handler while restarting
			thread.handlerMu.Lock()
			thread.state.set(stateRestarting)
			close(thread.drainChan)
			go func(thread *phpThread) {
				thread.state.waitFor(stateYielding)
				ready.Done()
			}(thread)
		}
	}
	ready.Wait()
	for _, worker := range workers {
		for _, thread := range worker.threads {
			thread.drainChan = make(chan struct{})
			thread.state.set(stateReady)
			thread.handlerMu.Unlock()
		}
		worker.threadMutex.RUnlock()
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
	defer worker.threadMutex.RUnlock()

	return len(worker.threads)
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
