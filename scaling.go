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
	// only allow scaling threads if requests were stalled for longer than this time
	allowedStallTime = 10 * time.Millisecond
	// time to wait after scaling a thread to prevent spending too many resources on scaling
	scaleBlockTime = 100 * time.Millisecond
	// check for and stop idle threads every x seconds
	downScaleCheckTime = 5 * time.Second
	// if an autoscaled thread has been waiting for longer than this time, terminate it
	maxThreadIdleTime = 5 * time.Second
	// amount of threads that can be stopped at once
	maxTerminationCount = 10
)

var (
	autoScaledThreads = []*phpThread{}
	scalingMu         = new(sync.RWMutex)
	isAutoScaling     = atomic.Bool{}
)

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

func initAutoScaling() {
	autoScaledThreads = []*phpThread{}
	isAutoScaling.Store(false)
	timer := time.NewTimer(downScaleCheckTime)
	doneChan := mainThread.done
	go func() {
		for {
			timer.Reset(downScaleCheckTime)
			select {
			case <-doneChan:
				return
			case <-timer.C:
				downScaleThreads()
			}
		}
	}()
}

// Add worker PHP threads automatically
// only add threads if requests were stalled long enough and no other scaling is in progress
func autoscaleWorkerThreads(worker *worker, timeSpentStalling time.Duration) {
	if timeSpentStalling < allowedStallTime || !isAutoScaling.CompareAndSwap(false, true) {
		return
	}

	count, err := AddWorkerThread(worker.fileName)
	worker.threadMutex.RLock()
	autoScaledThreads = append(autoScaledThreads, worker.threads[len(worker.threads)-1])
	worker.threadMutex.RUnlock()

	logger.Debug("worker thread autoscaling", zap.String("worker", worker.fileName), zap.Int("count", count), zap.Error(err))

	// wait a bit to prevent spending too much time on scaling
	time.Sleep(scaleBlockTime)
	isAutoScaling.Store(false)
}

// Add regular PHP threads automatically
// only add threads if requests were stalled long enough and no other scaling is in progress
func autoscaleRegularThreads(timeSpentStalling time.Duration) {
	if timeSpentStalling < allowedStallTime || !isAutoScaling.CompareAndSwap(false, true) {
		return
	}

	count, err := AddRegularThread()

	regularThreadMu.RLock()
	autoScaledThreads = append(autoScaledThreads, regularThreads[len(regularThreads)-1])
	regularThreadMu.RUnlock()

	logger.Debug("regular thread autoscaling", zap.Int("count", count), zap.Error(err))

	// wait a bit to prevent spending too much time on scaling
	time.Sleep(scaleBlockTime)
	isAutoScaling.Store(false)
}

func downScaleThreads() {
	stoppedThreadCount := 0
	scalingMu.Lock()
	defer scalingMu.Unlock()
	for i := len(autoScaledThreads) - 1; i >= 0; i-- {
		thread := autoScaledThreads[i]

		// remove the thread if it's reserved
		if thread.state.is(stateReserved) {
			autoScaledThreads = append(autoScaledThreads[:i], autoScaledThreads[i+1:]...)
			continue
		}
		if stoppedThreadCount > maxTerminationCount || thread.waitingSince == 0 {
			continue
		}

		// convert threads to inactive if they have been idle for too long
		threadIdleTime := time.Now().UnixMilli() - thread.waitingSince
		if thread.state.is(stateReady) && threadIdleTime > maxThreadIdleTime.Milliseconds() {
			logger.Debug("auto-converting thread to inactive", zap.Int("threadIndex", thread.threadIndex))
			convertToInactiveThread(thread)
			stoppedThreadCount++

			continue
		}

		// if threads are already inactive, shut them down
		if thread.state.is(stateInactive) && threadIdleTime > maxThreadIdleTime.Milliseconds() {
			logger.Debug("auto-stopping thread", zap.Int("threadIndex", thread.threadIndex))
			thread.shutdown()
			stoppedThreadCount++
			continue
		}
	}
}
