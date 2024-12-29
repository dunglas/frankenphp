package frankenphp

//#include "frankenphp.h"
import "C"
import (
	"errors"
	"net/http"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
)

// TODO: make speed of scaling dependant on CPU count?
const (
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

type scalingStrategy interface {
	apply(requestChan chan *http.Request, r *http.Request, scaleFunc func())
}

type scalingStrategyNormal struct {
	minStallTime time.Duration
	blockScaling atomic.Bool
}
type scalingStrategyNone struct{}

var (
	activeScalingStrategy scalingStrategy
	autoScaledThreads     = []*phpThread{}
	scalingMu             = new(sync.RWMutex)
	cpuCount              = runtime.NumCPU()

	MaxThreadsReachedError      = errors.New("max amount of overall threads reached")
	CannotRemoveLastThreadError = errors.New("cannot remove last thread")
	WorkerNotFoundError         = errors.New("worker not found for given filename")
)

// when scaling is disabled, just send the request to the channel
func (s scalingStrategyNone) apply(requestChan chan *http.Request, r *http.Request, scaleFunc func()) {
	requestChan <- r
}

// start a timer that triggers autoscaling
// after triggering autoscaling, double the timer's length
func (s scalingStrategyNormal) apply(requestChan chan *http.Request, r *http.Request, scaleFunc func()) {
	timeout := s.minStallTime
	timer := time.NewTimer(timeout)

	for {
		select {
		case requestChan <- r:
			timer.Stop()
			return
		case <-timer.C:
			if s.blockScaling.CompareAndSwap(false, true) {
				go func() {
					scaleFunc()
					s.blockScaling.Store(false)
				}()
			}
			timeout *= 2
			timer.Reset(timeout)
		}
	}
}

func initAutoScaling(numThreads int, maxThreads int, s ScalingStrategy) {
	if maxThreads <= numThreads || s == ScalingStrategyNone {
		activeScalingStrategy = scalingStrategyNone{}
		return
	}
	activeScalingStrategy = scalingStrategyNormal{
		minStallTime: 5 * time.Millisecond,
		blockScaling: atomic.Bool{},
	}
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
	activeScalingStrategy = scalingStrategyNone{}
	scalingMu.Unlock()
}

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

// remove the last regular thread
func RemoveRegularThread() (int, error) {
	scalingMu.Lock()
	defer scalingMu.Unlock()
	err := removeRegularThread()
	return countRegularThreads(), err
}

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

// turn the first inactive/reserved thread into a worker thread
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

func addWorkerThread(worker *worker) (*phpThread, error) {
	thread := getInactivePHPThread()
	if thread == nil {
		return nil, MaxThreadsReachedError
	}
	convertToWorkerThread(thread, worker)
	thread.state.waitFor(stateReady, stateShuttingDown, stateReserved)
	return thread, nil
}

// remove the last worker thread
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

// Add a worker PHP threads automatically
func scaleWorkerThreads(worker *worker) {
	scalingMu.Lock()
	defer scalingMu.Unlock()

	// TODO: is there an easy way to check if we are reaching memory limits?

	if !mainThread.state.is(stateReady) || !probeCPUs(cpuProbeTime) {
		return
	}

	thread, err := addWorkerThread(worker)
	if err != nil {
		logMaxThreadsReachedWarning(zap.String("worker", worker.fileName), zap.Error(err))
		return
	}

	autoScaledThreads = append(autoScaledThreads, thread)
}

// Add a regular PHP thread automatically
func scaleRegularThreads() {
	scalingMu.Lock()
	defer scalingMu.Unlock()

	if !mainThread.state.is(stateReady) || !probeCPUs(cpuProbeTime) {
		return
	}

	thread, err := addRegularThread()
	if err != nil {
		logMaxThreadsReachedWarning(zap.Error(err))
		return
	}

	autoScaledThreads = append(autoScaledThreads, thread)
}

// Check all threads and remove those that have been inactive for too long
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

	// note: clock_gettime is a POSIX function
	// on Windows we'd need to use QueryPerformanceCounter instead
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

// only log the maximum amount of threads reached warning once per minute
var lastMaxThreadsWarning = time.Time{}

func logMaxThreadsReachedWarning(zapFields ...zap.Field) {
	if lastMaxThreadsWarning.Add(time.Minute).Before(time.Now()) {
		logger.Warn("could not increase max_threads, consider raising this limit", zapFields...)
		lastMaxThreadsWarning = time.Now()
	}
}
