package frankenphp

//#include "frankenphp.h"
//#include <sys/resource.h>
import "C"
import (
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dunglas/frankenphp/internal/cpu"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const (
	// requests have to be stalled for at least this amount of time before scaling
	minStallTime = 5 * time.Millisecond
	// time to check for CPU usage before scaling a single thread
	cpuProbeTime = 120 * time.Millisecond
	// do not scale over this amount of CPU usage
	maxCpuUsageForScaling = 0.8
	// upscale stalled threads every x milliseconds
	upscaleCheckTime = 100 * time.Millisecond
	// downscale idle threads every x seconds
	downScaleCheckTime = 5 * time.Second
	// max amount of threads stopped in one iteration of downScaleCheckTime
	maxTerminationCount = 10
	// autoscaled threads waiting for longer than this time are downscaled
	maxThreadIdleTime = 5 * time.Second
)

var (
	scaleChan         chan *FrankenPHPContext
	autoScaledThreads = []*phpThread{}
	scalingMu         = new(sync.RWMutex)
	disallowScaling   = atomic.Bool{}

	MaxThreadsReachedError      = errors.New("max amount of overall threads reached")
	CannotRemoveLastThreadError = errors.New("cannot remove last thread")
	WorkerNotFoundError         = errors.New("worker not found for given filename")
)

func initAutoScaling(mainThread *phpMainThread) {
	if mainThread.maxThreads <= mainThread.numThreads {
		scaleChan = nil
		return
	}

	scalingMu.Lock()
	scaleChan = make(chan *FrankenPHPContext)
	maxScaledThreads := mainThread.maxThreads - mainThread.numThreads
	autoScaledThreads = make([]*phpThread, 0, maxScaledThreads)
	scalingMu.Unlock()

	go startUpscalingThreads(maxScaledThreads, scaleChan, mainThread.done)
	go startDownScalingThreads(mainThread.done)
}

func drainAutoScaling() {
	scalingMu.Lock()
	if c := logger.Check(zapcore.DebugLevel, "shutting down autoscaling"); c != nil {
		c.Write(zap.Int("autoScaledThreads", len(autoScaledThreads)))
	}
	scalingMu.Unlock()
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

func addWorkerThread(worker *worker) (*phpThread, error) {
	thread := getInactivePHPThread()
	if thread == nil {
		return nil, MaxThreadsReachedError
	}
	convertToWorkerThread(thread, worker)
	thread.state.waitFor(stateReady, stateShuttingDown, stateReserved)
	return thread, nil
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

// scaleWorkerThread adds a worker PHP thread automatically
func scaleWorkerThread(worker *worker) {
	scalingMu.Lock()
	defer scalingMu.Unlock()

	if !mainThread.state.is(stateReady) {
		return
	}

	// probe CPU usage before scaling
	if !cpu.ProbeCPUs(cpuProbeTime, maxCpuUsageForScaling, mainThread.done) {
		return
	}

	thread, err := addWorkerThread(worker)
	if err != nil {
		if c := logger.Check(zapcore.WarnLevel, "could not increase max_threads, consider raising this limit"); c != nil {
			c.Write(zap.String("worker", worker.fileName), zap.Error(err))
		}
		return
	}

	autoScaledThreads = append(autoScaledThreads, thread)
}

// scaleRegularThread adds a regular PHP thread automatically
func scaleRegularThread() {
	scalingMu.Lock()
	defer scalingMu.Unlock()

	if !mainThread.state.is(stateReady) {
		return
	}

	// probe CPU usage before scaling
	if !cpu.ProbeCPUs(cpuProbeTime, maxCpuUsageForScaling, mainThread.done) {
		return
	}

	thread, err := addRegularThread()
	if err != nil {
		if c := logger.Check(zapcore.WarnLevel, "could not increase max_threads, consider raising this limit"); c != nil {
			c.Write(zap.Error(err))
		}
		return
	}

	autoScaledThreads = append(autoScaledThreads, thread)
}

func startUpscalingThreads(maxScaledThreads int, scale chan *FrankenPHPContext, done chan struct{}) {
	for {
		scalingMu.Lock()
		scaledThreadCount := len(autoScaledThreads)
		scalingMu.Unlock()
		if scaledThreadCount >= maxScaledThreads {
			// we have reached max_threads, check again later
			select {
			case <-done:
				return
			case <-time.After(downScaleCheckTime):
				continue
			}
		}

		select {
		case fc := <-scale:
			timeSinceStalled := time.Since(fc.startedAt)

			// if the request has not been stalled long enough, wait and repeat
			if timeSinceStalled < minStallTime {
				select {
				case <-done:
					return
				case <-time.After(minStallTime - timeSinceStalled):
					continue
				}
			}

			// if the request has been stalled long enough, scale
			if worker, ok := workers[fc.scriptFilename]; ok {
				scaleWorkerThread(worker)
			} else {
				scaleRegularThread()
			}
		case <-done:
			return
		}
	}
}

func startDownScalingThreads(done chan struct{}) {
	for {
		select {
		case <-done:
			return
		case <-time.After(downScaleCheckTime):
			deactivateThreads()
		}
	}
}

// deactivateThreads checks all threads and removes those that have been inactive for too long
func deactivateThreads() {
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
			if c := logger.Check(zapcore.DebugLevel, "auto-converting thread to inactive"); c != nil {
				c.Write(zap.Int("threadIndex", thread.threadIndex))
			}
			convertToInactiveThread(thread)
			stoppedThreadCount++
			autoScaledThreads = append(autoScaledThreads[:i], autoScaledThreads[i+1:]...)

			continue
		}

		// TODO: Completely stopping threads is more memory efficient
		// Some PECL extensions like #1296 will prevent threads from fully stopping (they leak memory)
		// Reactivate this if there is a better solution or workaround
		//if thread.state.is(stateInactive) && waitTime > maxThreadIdleTime.Milliseconds() {
		//	if c := logger.Check(zapcore.DebugLevel, "auto-stopping thread"); c != nil {
		//    c.Write(zap.Int("threadIndex", thread.threadIndex))
		//  }
		//	thread.shutdown()
		//	stoppedThreadCount++
		//	autoScaledThreads = append(autoScaledThreads[:i], autoScaledThreads[i+1:]...)
		//	continue
		//}
	}
}
