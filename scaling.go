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

// TODO: make speed of scaling dependant on CPU count?
const (
	// only allow scaling threads if requests were stalled for longer than this time
	allowedStallTime = 10 * time.Millisecond
	// time to wait after scaling a thread to prevent spending too many resources on scaling
	scaleBlockTime = 100 * time.Millisecond
	// check for and stop idle threads every x seconds
	downScaleCheckTime = 5 * time.Second
	// if an autoscaled thread has been waiting for longer than this time, terminate it
	maxThreadIdleTime = 5 * time.Second
	// if PHP threads are using more than this percentage of CPU, do not scale
	maxCpuPotential = 0.85
	// amount of threads that can be stopped at once
	maxTerminationCount = 10
)

var (
	autoScaledThreads    = []*phpThread{}
	scalingMu            = new(sync.RWMutex)
	blockAutoScaling     = atomic.Bool{}
	cpuCount             = runtime.NumCPU()
	allThreadsCpuPercent float64
	cpuMutex             sync.Mutex
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

	// TODO: instead of starting new threads, would it make sense to convert idle ones?
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

func initAutoScaling(numThreads int, maxThreads int) {
	if maxThreads <= numThreads {
		blockAutoScaling.Store(true)
		return
	}
	autoScaledThreads = make([]*phpThread, 0, maxThreads-numThreads)
	blockAutoScaling.Store(false)
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
func autoscaleWorkerThreads(worker *worker, timeSpentStalling time.Duration) {

	// first check if time spent waiting for a thread was above the allowed threshold
	if timeSpentStalling < allowedStallTime || !blockAutoScaling.CompareAndSwap(false, true) {
		return
	}

	threadCount := worker.countThreads()
	if cpuCoresAreBusy(threadCount) {
		logger.Debug("not autoscaling", zap.String("worker", worker.fileName), zap.Int("count", threadCount))
		time.Sleep(scaleBlockTime)
		blockAutoScaling.Store(false)
		return
	}

	_, err := AddWorkerThread(worker.fileName)
	if err != nil {
		logger.Debug("could not add worker thread", zap.String("worker", worker.fileName), zap.Error(err))
	}

	scalingMu.Lock()
	autoScaledThreads = append(autoScaledThreads, worker.threads[len(worker.threads)-1])
	scalingMu.Unlock()

	// wait a bit to prevent spending too much time on scaling
	time.Sleep(scaleBlockTime)
	blockAutoScaling.Store(false)
}

// Add regular PHP threads automatically
// Only add threads if requests were stalled long enough and no other scaling is in progress
func autoscaleRegularThreads(timeSpentStalling time.Duration) {
	if timeSpentStalling < allowedStallTime || !blockAutoScaling.CompareAndSwap(false, true) {
		return
	}

	count, err := AddRegularThread()
	scalingMu.Lock()
	autoScaledThreads = append(autoScaledThreads, regularThreads[len(regularThreads)-1])
	scalingMu.Unlock()

	logger.Debug("regular thread autoscaling", zap.Int("count", count), zap.Error(err))

	// wait a bit to prevent spending too much time on scaling
	time.Sleep(scaleBlockTime)
	blockAutoScaling.Store(false)
}

func downScaleThreads() {
	stoppedThreadCount := 0
	scalingMu.Lock()
	defer scalingMu.Unlock()
	for i := len(autoScaledThreads) - 1; i >= 0; i-- {
		thread := autoScaledThreads[i]

		// the thread might have been stopped otherwise, remove it
		if thread.state.is(stateReserved) {
			autoScaledThreads = append(autoScaledThreads[:i], autoScaledThreads[i+1:]...)
			continue
		}

		waitTime := thread.state.waitTime()
		if stoppedThreadCount > maxTerminationCount || waitTime == 0 {
			continue
		}

		// convert threads to inactive if they have been idle for too long
		if thread.state.is(stateReady) && waitTime > maxThreadIdleTime.Milliseconds() {
			logger.Debug("auto-converting thread to inactive", zap.Int("threadIndex", thread.threadIndex))
			convertToInactiveThread(thread)
			stoppedThreadCount++

			continue
		}

		// if threads are already inactive, shut them down
		if thread.state.is(stateInactive) && waitTime > maxThreadIdleTime.Milliseconds() {
			logger.Debug("auto-stopping thread", zap.Int("threadIndex", thread.threadIndex))
			thread.shutdown()
			stoppedThreadCount++
			autoScaledThreads = append(autoScaledThreads[:i], autoScaledThreads[i+1:]...)
			continue
		}
	}
}

// threads spend a certain % of time on CPU cores and a certain % waiting for IO
// this function tracks the CPU usage and weighs it against previous requests
func trackCpuUsage(cpuPercent float64) {
	cpuMutex.Lock()
	allThreadsCpuPercent = (allThreadsCpuPercent*99 + cpuPercent) / 100
	cpuMutex.Unlock()
}

// threads track how much time they spend on CPU cores
// cpuPotential is the average amount of time threads spend on CPU cores * the number of threads
// example: 10 threads that spend 10% of their time on CPU cores and 90% waiting for IO, would have a potential of 100%
// only scale if the potential is below a threshold
// if the potential is too high, then requests are stalled because of CPU usage, not because of IO
func cpuCoresAreBusy(threadCount int) bool {
	cpuMutex.Lock()
	cpuPotential := allThreadsCpuPercent * float64(threadCount) / float64(cpuCount)
	cpuMutex.Unlock()
	return cpuPotential > maxCpuPotential
}
