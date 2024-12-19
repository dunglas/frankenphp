package frankenphp

//#include "frankenphp.h"
import "C"
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
	// the amount of time to check for CPU usage before scaling
	cpuProbeTime = 100 * time.Millisecond
	// if PHP threads are using more than this ratio of the CPU, do not scale
	maxCpuUsageForScaling = 0.8
	// check if threads should be stopped every x seconds
	downScaleCheckTime = 5 * time.Second
	// amount of threads that can be stopped in one iteration of downScaleCheckTime
	maxTerminationCount = 10
	// if an autoscaled thread has been waiting for longer than this time, terminate it
	maxThreadIdleTime = 5 * time.Second
)

var (
	autoScaledThreads = []*phpThread{}
	scalingMu         = new(sync.RWMutex)
	blockAutoScaling  = atomic.Bool{}
	cpuCount          = runtime.NumCPU()
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
	defer blockAutoScaling.Store(false)

	// TODO: is there an easy way to check if we are reaching memory limits?

	if probeIfCpusAreBusy(cpuProbeTime) {
		logger.Debug("cpu is busy, not autoscaling", zap.String("worker", worker.fileName))
		return
	}

	count, err := AddWorkerThread(worker.fileName)
	if err != nil {
		logger.Debug("could not add worker thread", zap.String("worker", worker.fileName), zap.Int("count", count), zap.Error(err))
	}

	scalingMu.Lock()
	autoScaledThreads = append(autoScaledThreads, worker.threads[len(worker.threads)-1])
	scalingMu.Unlock()
}

// Add regular PHP threads automatically
func autoscaleRegularThreads(timeSpentStalling time.Duration) {

	// first check if time spent waiting for a thread was above the allowed threshold
	if timeSpentStalling < allowedStallTime || !blockAutoScaling.CompareAndSwap(false, true) {
		return
	}
	defer blockAutoScaling.Store(false)

	if probeIfCpusAreBusy(cpuProbeTime) {
		logger.Debug("cpu is busy, not autoscaling")
		return
	}

	count, err := AddRegularThread()
	scalingMu.Lock()
	autoScaledThreads = append(autoScaledThreads, regularThreads[len(regularThreads)-1])
	scalingMu.Unlock()

	logger.Debug("regular thread autoscaling", zap.Int("count", count), zap.Error(err))
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

func readMemory() {
	return
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	fmt.Printf("Total allocated memory: %d bytes\n", mem.TotalAlloc)
	fmt.Printf("Number of memory allocations: %d\n", mem.Mallocs)
}

// probe the CPU usage of the process
// if CPUs are not busy, most threads are likely waiting for I/O, so we should scale
// if CPUs are already busy we won't gain much by scaling and want to avoid the overhead of doing so
// keep in mind that this will only probe CPU usage by PHP Threads
// time spent by the go runtime or other processes is not considered
func probeIfCpusAreBusy(sleepTime time.Duration) bool {
	cpuUsage := float64(C.frankenphp_probe_cpu(C.int(cpuCount), C.int(sleepTime.Milliseconds())))

	logger.Warn("CPU usage", zap.Float64("usage", cpuUsage))
	return cpuUsage > maxCpuUsageForScaling
}
