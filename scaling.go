package frankenphp

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
)

const (
	// only allow scaling threads if they were stalled longer than this time
	allowedStallTime = 10 * time.Millisecond
	// time to wait after scaling a thread to prevent scaling too fast
	scaleBlockTime = 100 * time.Millisecond
	// time to wait between checking for idle threads
	downScaleCheckTime = 5 * time.Second
	// max time a thread can be idle before being stopped or converted to inactive
	maxThreadIdleTime = 5 * time.Second
	// amount of threads that can be stopped in one downScaleCheckTime iteration
	amountOfThreadsStoppedAtOnce = 10
)

var scalingMu = new(sync.RWMutex)
var isAutoScaling = atomic.Bool{}

func initAutoScaling() {
	timer := time.NewTimer(downScaleCheckTime)
	doneChan := mainThread.done
	go func() {
		for {
			timer.Reset(downScaleCheckTime)
			select {
			case <-doneChan:
				return
			case <-timer.C:
				stopIdleThreads()
			}
		}
	}()
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

// worker thread autoscaling
func requestNewWorkerThread(worker *worker, timeSpentStalling time.Duration) {
	// ignore requests that have been stalled for an acceptable amount of time
	if timeSpentStalling < allowedStallTime || !isAutoScaling.CompareAndSwap(false, true) {
		return
	}

	count, err := AddWorkerThread(worker.fileName)

	logger.Debug("worker thread autoscaling", zap.String("worker", worker.fileName), zap.Int("count", count), zap.Error(err))

	// wait a bit to prevent spending too much time on scaling
	time.Sleep(scaleBlockTime)
	isAutoScaling.Store(false)
}

func stopIdleThreads() {
	stoppedThreadCount := 0
	for i := len(phpThreads) - 1; i >= 0; i-- {
		thread := phpThreads[i]
		if stoppedThreadCount > amountOfThreadsStoppedAtOnce || thread.isProtected || thread.waitingSince == 0 {
			continue
		}
		waitingMilliseconds := time.Now().UnixMilli() - thread.waitingSince

		// convert threads to inactive first
		if thread.state.is(stateReady) && waitingMilliseconds > maxThreadIdleTime.Milliseconds() {
			convertToInactiveThread(thread)
			stoppedThreadCount++
			continue
		}

		// if threads are already inactive, shut them down
		if thread.state.is(stateInactive) && waitingMilliseconds > maxThreadIdleTime.Milliseconds() {
			thread.shutdown()
			stoppedThreadCount++
			continue
		}
	}
}
