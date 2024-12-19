package frankenphp

//#include "frankenphp.h"
import "C"
import (
	"errors"
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
	cpuProbeTime = 50 * time.Millisecond
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
	_, err := addRegularThread()
	return countRegularThreads(), err
}

func addRegularThread() (*phpThread, error) {
	thread := getInactivePHPThread()
	if thread == nil {
		return nil, errors.New("max amount of overall threads reached")
	}
	convertToRegularThread(thread)
	return thread, nil
}

func RemoveRegularThread() (int, error) {
	scalingMu.Lock()
	defer scalingMu.Unlock()
	err := removeRegularThread()
	return countRegularThreads(), err
}

// remove the last regular thread
func removeRegularThread() error {
	regularThreadMu.RLock()
	if len(regularThreads) <= 1 {
		regularThreadMu.RUnlock()
		return errors.New("cannot remove last thread")
	}
	thread := regularThreads[len(regularThreads)-1]
	regularThreadMu.RUnlock()
	thread.shutdown()
	return nil
}

func AddWorkerThread(workerFileName string) (int, error) {
	worker, ok := workers[workerFileName]
	if !ok {
		return 0, errors.New("worker not found")
	}
	scalingMu.Lock()
	defer scalingMu.Unlock()
	_, err := addWorkerThread(worker)
	return worker.countThreads(), err
}

// turn the first inactive/reserved thread into a worker thread
func addWorkerThread(worker *worker) (*phpThread, error) {
	thread := getInactivePHPThread()
	if thread == nil {
		return nil, errors.New("max amount of overall threads reached")
	}
	convertToWorkerThread(thread, worker)
	return thread, nil
}

func RemoveWorkerThread(workerFileName string) (int, error) {
	worker, ok := workers[workerFileName]
	if !ok {
		return 0, errors.New("worker not found")
	}
	scalingMu.Lock()
	defer scalingMu.Unlock()
	err := removeWorkerThread(worker)

	return worker.countThreads(), err
}

// remove the last worker thread
func removeWorkerThread(worker *worker) error {
	worker.threadMutex.RLock()
	if len(worker.threads) <= 1 {
		worker.threadMutex.RUnlock()
		return errors.New("cannot remove last thread")
	}
	thread := worker.threads[len(worker.threads)-1]
	worker.threadMutex.RUnlock()
	thread.shutdown()

	return nil
}

func initAutoScaling(numThreads int, maxThreads int) {
	if maxThreads <= numThreads {
		blockAutoScaling.Store(true)
		return
	}
	blockAutoScaling.Store(false)
	autoScaledThreads = make([]*phpThread, 0, maxThreads-numThreads)
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

func drainAutoScaling() {
	scalingMu.Lock()
	blockAutoScaling.Store(true)
	scalingMu.Unlock()
}

// Add worker PHP threads automatically
func autoscaleWorkerThreads(worker *worker, timeSpentStalling time.Duration) {
	// first check if time spent waiting for a thread was above the allowed threshold
	if timeSpentStalling < allowedStallTime || !blockAutoScaling.CompareAndSwap(false, true) {
		return
	}
	scalingMu.Lock()
	defer scalingMu.Unlock()
	defer blockAutoScaling.Store(false)

	// TODO: is there an easy way to check if we are reaching memory limits?

	if !probeCPUs(cpuProbeTime) {
		logger.Debug("cpu is busy, not autoscaling", zap.String("worker", worker.fileName))
		return
	}

	thread, err := addWorkerThread(worker)
	if err != nil {
		logger.Debug("could not add worker thread", zap.String("worker", worker.fileName), zap.Error(err))
		return
	}

	autoScaledThreads = append(autoScaledThreads, thread)
}

// Add regular PHP threads automatically
func autoscaleRegularThreads(timeSpentStalling time.Duration) {
	// first check if time spent waiting for a thread was above the allowed threshold
	if timeSpentStalling < allowedStallTime || !blockAutoScaling.CompareAndSwap(false, true) {
		return
	}
	scalingMu.Lock()
	defer scalingMu.Unlock()
	defer blockAutoScaling.Store(false)

	if !probeCPUs(cpuProbeTime) {
		logger.Debug("cpu is busy, not autoscaling")
		return
	}

	thread, err := addRegularThread()
	if err != nil {
		logger.Debug("could not add regular thread", zap.Error(err))
		return
	}

	autoScaledThreads = append(autoScaledThreads, thread)
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

// probe the CPU usage of all PHP Threads
// if CPUs are not busy, most threads are likely waiting for I/O, so we should scale
// if CPUs are already busy we won't gain much by scaling and want to avoid the overhead of doing so
// time spent by the go runtime or other processes is not considered
func probeCPUs(probeTime time.Duration) bool {
	var start, end, cpuStart, cpuEnd C.struct_timespec

	C.clock_gettime(C.CLOCK_MONOTONIC, &start)
	C.clock_gettime(C.CLOCK_PROCESS_CPUTIME_ID, &cpuStart)

	timer := time.NewTimer(probeTime)
	select {
	case <-mainThread.done:
		return false
	case <-timer.C:
	}

	C.clock_gettime(C.CLOCK_MONOTONIC, &end)
	C.clock_gettime(C.CLOCK_PROCESS_CPUTIME_ID, &cpuEnd)

	elapsedTime := float64(end.tv_sec-start.tv_sec)*1e9 + float64(end.tv_nsec-start.tv_nsec)
	elapsedCpuTime := float64(cpuEnd.tv_sec-cpuStart.tv_sec)*1e9 + float64(cpuEnd.tv_nsec-cpuStart.tv_nsec)
	cpuUsage := elapsedCpuTime / elapsedTime / float64(cpuCount)

	// TODO: remove unnecessary debug messages
	logger.Debug("CPU usage", zap.Float64("cpuUsage", cpuUsage))

	return cpuUsage < maxCpuUsageForScaling
}
