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
	// scale threads if requests stall this amount of time
	allowedStallTime = 10 * time.Millisecond
	// time to check for CPU usage before scaling a single thread
	cpuProbeTime = 40 * time.Millisecond
	// do not scale over this amount of CPU usage
	maxCpuUsageForScaling = 0.8
	// downscale idle threads every x seconds
	downScaleCheckTime = 5 * time.Second
	// max amount of threads stopped in one iteration of downScaleCheckTime
	maxTerminationCount = 10
	// autoscaled threads waiting for longer than this time are downscaled
	maxThreadIdleTime = 5 * time.Second
)

var (
	autoScaledThreads = []*phpThread{}
	scalingMu         = new(sync.RWMutex)
	blockAutoScaling  = atomic.Bool{}
	cpuCount          = runtime.NumCPU()

	MaxThreadsReachedError      = errors.New("max amount of overall threads reached")
	CannotRemoveLastThreadError = errors.New("cannot remove last thread")
	WorkerNotFoundError         = errors.New("worker not found for given filename")
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
		return nil, MaxThreadsReachedError
	}
	convertToRegularThread(thread)
	thread.state.waitFor(stateReady, stateShuttingDown, stateReserved)
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
		return CannotRemoveLastThreadError
	}
	thread := regularThreads[len(regularThreads)-1]
	regularThreadMu.RUnlock()
	thread.shutdown()
	return nil
}

func AddWorkerThread(workerFileName string) (int, error) {
	worker, ok := workers[workerFileName]
	if !ok {
		return 0, WorkerNotFoundError
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
		return nil, MaxThreadsReachedError
	}
	convertToWorkerThread(thread, worker)
	thread.state.waitFor(stateReady, stateShuttingDown, stateReserved)
	return thread, nil
}

func RemoveWorkerThread(workerFileName string) (int, error) {
	worker, ok := workers[workerFileName]
	if !ok {
		return 0, WorkerNotFoundError
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
		return CannotRemoveLastThreadError
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
func autoscaleWorkerThreads(worker *worker) {
	scalingMu.Lock()
	defer scalingMu.Unlock()

	// TODO: is there an easy way to check if we are reaching memory limits?

	if !probeCPUs(cpuProbeTime) {
		logger.Debug("cpu is busy, not autoscaling", zap.String("worker", worker.fileName))
		return
	}

	depth := metrics.GetWorkerQueueDepth(worker.fileName)

	if depth <= 0 {
		return
	}

	thread, err := addWorkerThread(worker)
	if err != nil {
		logger.Info("could not increase the amount of threads handling requests", zap.String("worker", worker.fileName), zap.Error(err))
		return
	}

	autoScaledThreads = append(autoScaledThreads, thread)
}

// Add regular PHP threads automatically
func autoscaleRegularThreads() {
	scalingMu.Lock()
	defer scalingMu.Unlock()

	if !probeCPUs(cpuProbeTime) {
		logger.Debug("cpu is busy, not autoscaling")
		return
	}

	depth := metrics.GetQueueDepth()
	if depth <= 0 {
		return
	}

	thread, err := addRegularThread()
	if err != nil {
		logger.Info("could not increase the amount of threads handling requests", zap.Error(err))
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

	// TODO: validate cross-platform compatibility
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

	return cpuUsage < maxCpuUsageForScaling
}
