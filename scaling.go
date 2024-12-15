package frankenphp

import (
	"errors"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
)

var scalingMu = new(sync.RWMutex)
var isAutoScaling = atomic.Bool{}
var cpuCount = runtime.NumCPU()

func initAutoScaling() {
	return
	timer := time.NewTimer(5 * time.Second)
	for {
		timer.Reset(5 * time.Second)
		select {
		case <-mainThread.done:
			return
		case <-timer.C:
			autoScaleThreads()
		}
	}
}

// turn the first inactive/reserved thread into a regular thread
func AddRegularThread() (int, error) {
	scalingMu.Lock()
	defer scalingMu.Unlock()
	thread := getInactivePHPThread()
	if thread == nil {
		return countRegularThreads(), fmt.Errorf("max amount of overall threads reached: %d", len(phpThreads))
	}
	convertToRegularThread(thread)
	return countRegularThreads(), nil
}

// remove the last regular thread
func RemoveRegularThread() (int, error) {
	scalingMu.Lock()
	defer scalingMu.Unlock()
	regularThreadMu.RLock()
	if len(regularThreads) <= 1 {
		regularThreadMu.RUnlock()
		return 1, errors.New("cannot remove last thread")
	}
	thread := regularThreads[len(regularThreads)-1]
	regularThreadMu.RUnlock()
	thread.shutdown()
	return countRegularThreads(), nil
}

// turn the first inactive/reserved thread into a worker thread
func AddWorkerThread(workerFileName string) (int, error) {
	scalingMu.Lock()
	defer scalingMu.Unlock()
	worker, ok := workers[workerFileName]
	if !ok {
		return 0, errors.New("worker not found")
	}
	thread := getInactivePHPThread()
	if thread == nil {
		count := worker.countThreads()
		return count, fmt.Errorf("max amount of threads reached: %d", count)
	}
	convertToWorkerThread(thread, worker)
	return worker.countThreads(), nil
}

// remove the last worker thread
func RemoveWorkerThread(workerFileName string) (int, error) {
	scalingMu.Lock()
	defer scalingMu.Unlock()
	worker, ok := workers[workerFileName]
	if !ok {
		return 0, errors.New("worker not found")
	}

	worker.threadMutex.RLock()
	if len(worker.threads) <= 1 {
		worker.threadMutex.RUnlock()
		return 1, errors.New("cannot remove last thread")
	}
	thread := worker.threads[len(worker.threads)-1]
	worker.threadMutex.RUnlock()
	thread.shutdown()

	return worker.countThreads(), nil
}

var averageStallPercent float64 = 0.0
var stallMu = new(sync.Mutex)
var stallTime = 0

const minStallTimeMicroseconds = 10_000

func requestNewWorkerThread(worker *worker, timeSpentStalling int64, timeSpentTotal int64) {
	// ignore requests that have been stalled for an acceptable amount of time
	if timeSpentStalling < minStallTimeMicroseconds {
		return
	}
	// percent of time the request spent waiting for a thread
	stalledThisRequest := float64(timeSpentStalling) / float64(timeSpentTotal)

	// weigh the change to the average stall-time by the amount of handling threads
	numWorkers := float64(worker.countThreads())
	stallMu.Lock()
	averageStallPercent = (averageStallPercent*(numWorkers-1.0) + stalledThisRequest) / numWorkers
	stallMu.Unlock()

	// if we are only being stalled by a small amount, do not scale
	//logger.Info("stalling", zap.Float64("percent", averageStallPercent))
	if averageStallPercent < 0.66 {
		return
	}

	// prevent multiple auto-scaling attempts
	if !isAutoScaling.CompareAndSwap(false, true) {
		return
	}

	logger.Debug("scaling up worker thread", zap.String("worker", worker.fileName))

	// it does not matter here if adding a thread is successful or not
	_, _ = AddWorkerThread(worker.fileName)

	// wait a bit to prevent spending too much time on scaling
	time.Sleep(100 * time.Millisecond)
	isAutoScaling.Store(false)
}

func autoScaleThreads() {
	for i := len(phpThreads) - 1; i >= 0; i-- {
		thread := phpThreads[i]
		if thread.isProtected {
			continue
		}
		if thread.state.is(stateReady) && time.Now().UnixMilli()-thread.waitingSince > 5000 {
			convertToInactiveThread(thread)
			continue
		}
		if thread.state.is(stateInactive) && time.Now().UnixMilli()-thread.waitingSince > 5000 {
			thread.shutdown()
			continue
		}
	}
}
