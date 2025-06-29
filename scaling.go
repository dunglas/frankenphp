package frankenphp

//#include "frankenphp.h"
//#include <sys/resource.h>
import "C"
import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/dunglas/frankenphp/internal/cpu"
)

const (
	// requests have to be stalled for at least this amount of time before scaling
	minStallTime = 5 * time.Millisecond
	// time to check for CPU usage before scaling a single thread
	cpuProbeTime = 120 * time.Millisecond
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
	ErrMaxThreadsReached = errors.New("max amount of overall threads reached")

	scaleChan         chan *frankenPHPContext
	autoScaledThreads = []*phpThread{}
	scalingMu         = new(sync.RWMutex)
)

func initAutoScaling(mainThread *phpMainThread) {
	logger.Debug("initAutoScaling called")
	if mainThread.maxThreads <= mainThread.numThreads {
		scaleChan = nil
		logger.Debug("Auto-scaling disabled: maxThreads <= numThreads")
		return
	}

	scalingMu.Lock()
	scaleChan = make(chan *frankenPHPContext)
	maxScaledThreads := mainThread.maxThreads - mainThread.numThreads
	autoScaledThreads = make([]*phpThread, 0, maxScaledThreads)
	scalingMu.Unlock()
	logger.Debug("Auto-scaling initialized", slog.Int("maxScaledThreads", maxScaledThreads))

	go startUpscalingThreads(maxScaledThreads, scaleChan, mainThread.done)
	go startDownScalingThreads(mainThread.done)
}

func drainAutoScaling() {
	scalingMu.Lock()
	logger.LogAttrs(context.Background(), slog.LevelDebug, "shutting down autoscaling", slog.Int("autoScaledThreads", len(autoScaledThreads)))
	scalingMu.Unlock()
}

func addRegularThread() (*phpThread, error) {
	logger.Debug("addRegularThread called")
	thread := getInactivePHPThread()
	if thread == nil {
		logger.Warn("No inactive PHP thread available for regular thread")
		return nil, ErrMaxThreadsReached
	}
	convertToRegularThread(thread)
	thread.state.waitFor(stateReady, stateShuttingDown, stateReserved)
	logger.Debug("Regular thread added", slog.Int("threadIndex", thread.threadIndex))
	return thread, nil
}

func addWorkerThread(worker *worker) (*phpThread, error) {
	logger.Debug("addWorkerThread called", slog.String("workerName", worker.name))
	thread := getInactivePHPThread()
	if thread == nil {
		logger.Warn("No inactive PHP thread available for worker thread")
		return nil, ErrMaxThreadsReached
	}
	convertToWorkerThread(thread, worker)
	thread.state.waitFor(stateReady, stateShuttingDown, stateReserved)
	logger.Debug("Worker thread added", slog.Int("threadIndex", thread.threadIndex), slog.String("workerName", worker.name))
	return thread, nil
}

// scaleWorkerThread adds a worker PHP thread automatically
func scaleWorkerThread(worker *worker) {
	logger.Debug("scaleWorkerThread called", slog.String("workerName", worker.name))
	scalingMu.Lock()
	defer scalingMu.Unlock()

	if !mainThread.state.is(stateReady) {
		logger.Debug("Main thread not ready, skipping worker scaling")
		return
	}

	// probe CPU usage before scaling
	logger.Debug("Probing CPU usage before worker scaling")
	if !cpu.ProbeCPUs(cpuProbeTime, maxCpuUsageForScaling, mainThread.done) {
		logger.Debug("CPU usage too high or probe failed, skipping worker scaling")
		return
	}
	logger.Debug("CPU probe passed for worker scaling")

	thread, err := addWorkerThread(worker)
	if err != nil {
		logger.LogAttrs(context.Background(), slog.LevelWarn, "could not increase max_threads, consider raising this limit", slog.String("worker", worker.name), slog.Any("error", err))
		return
	}

	autoScaledThreads = append(autoScaledThreads, thread)
	logger.Debug("Worker thread scaled up", slog.Int("threadIndex", thread.threadIndex), slog.String("workerName", worker.name))
}

// scaleRegularThread adds a regular PHP thread automatically
func scaleRegularThread() {
	logger.Debug("scaleRegularThread called")
	scalingMu.Lock()
	defer scalingMu.Unlock()

	if !mainThread.state.is(stateReady) {
		logger.Debug("Main thread not ready, skipping regular thread scaling")
		return
	}

	// probe CPU usage before scaling
	logger.Debug("Probing CPU usage before regular thread scaling")
	if !cpu.ProbeCPUs(cpuProbeTime, maxCpuUsageForScaling, mainThread.done) {
		logger.Debug("CPU usage too high or probe failed, skipping regular thread scaling")
		return
	}
	logger.Debug("CPU probe passed for regular thread scaling")

	thread, err := addRegularThread()
	if err != nil {
		logger.LogAttrs(context.Background(), slog.LevelWarn, "could not increase max_threads, consider raising this limit", slog.Any("error", err))
		return
	}

	autoScaledThreads = append(autoScaledThreads, thread)
	logger.Debug("Regular thread scaled up", slog.Int("threadIndex", thread.threadIndex))
}

func startUpscalingThreads(maxScaledThreads int, scale chan *frankenPHPContext, done chan struct{}) {
	logger.Debug("startUpscalingThreads started", slog.Int("maxScaledThreads", maxScaledThreads))
	for {
		scalingMu.Lock()
		scaledThreadCount := len(autoScaledThreads)
		scalingMu.Unlock()
		if scaledThreadCount >= maxScaledThreads {
			// we have reached max_threads, check again later
			logger.Debug("Max scaled threads reached, waiting", slog.Int("scaledThreadCount", scaledThreadCount))
			select {
			case <-done:
				logger.Debug("Upscaling threads done signal received")
				return
			case <-time.After(downScaleCheckTime):
				logger.Debug("Upscaling threads waiting for downScaleCheckTime")
				continue
			}
		}

		select {
		case fc := <-scale:
			logger.Debug("Request received for upscaling", slog.String("scriptFilename", fc.scriptFilename))
			timeSinceStalled := time.Since(fc.startedAt)

			// if the request has not been stalled long enough, wait and repeat
			if timeSinceStalled < minStallTime {
				logger.Debug("Request not stalled long enough, waiting", slog.Duration("timeSinceStalled", timeSinceStalled))
				select {
				case <-done:
					logger.Debug("Upscaling threads done signal received during stall wait")
					return
				case <-time.After(minStallTime - timeSinceStalled):
					continue
				}
			}

			// if the request has been stalled long enough, scale
			if worker, ok := workers[getWorkerKey(fc.workerName, fc.scriptFilename)]; ok {
				logger.Debug("Scaling worker thread")
				scaleWorkerThread(worker)
			} else {
				logger.Debug("Scaling regular thread")
				scaleRegularThread()
			}
		case <-done:
			logger.Debug("Upscaling threads done signal received")
			return
		}
	}
}

func startDownScalingThreads(done chan struct{}) {
	logger.Debug("startDownScalingThreads started")
	for {
		select {
		case <-done:
			logger.Debug("Downscaling threads done signal received")
			return
		case <-time.After(downScaleCheckTime):
			logger.Debug("Downscaling threads checking for inactive threads")
			deactivateThreads()
		}
	}
}

// deactivateThreads checks all threads and removes those that have been inactive for too long
func deactivateThreads() {
	logger.Debug("deactivateThreads called")
	stoppedThreadCount := 0
	scalingMu.Lock()
	defer scalingMu.Unlock()
	for i := len(autoScaledThreads) - 1; i >= 0; i-- {
		thread := autoScaledThreads[i]

		// the thread might have been stopped otherwise, remove it
		if thread.state.is(stateReserved) {
			autoScaledThreads = append(autoScaledThreads[:i], autoScaledThreads[i+1:]...)
			logger.Debug("Reserved thread removed from autoScaledThreads", slog.Int("threadIndex", thread.threadIndex))
			continue
		}

		waitTime := thread.state.waitTime()
		if stoppedThreadCount > maxTerminationCount || waitTime == 0 {
			logger.Debug("Skipping thread deactivation", slog.Int("threadIndex", thread.threadIndex), slog.Int("stoppedThreadCount", stoppedThreadCount), slog.Int64("waitTime", waitTime))
			continue
		}

		// convert threads to inactive if they have been idle for too long
		if thread.state.is(stateReady) && waitTime > maxThreadIdleTime.Milliseconds() {
			logger.LogAttrs(context.Background(), slog.LevelDebug, "auto-converting thread to inactive", slog.Int("thread", thread.threadIndex))
			convertToInactiveThread(thread)
			stoppedThreadCount++
			autoScaledThreads = append(autoScaledThreads[:i], autoScaledThreads[i+1:]...)

			continue
		}

		// TODO: Completely stopping threads is more memory efficient
		// Some PECL extensions like #1296 will prevent threads from fully stopping (they leak memory)
		// Reactivate this if there is a better solution or workaround
		// if thread.state.is(stateInactive) && waitTime > maxThreadIdleTime.Milliseconds() {
		// 	logger.LogAttrs(nil, slog.LevelDebug, "auto-stopping thread", slog.Int("thread", thread.threadIndex))
		// 	thread.shutdown()
		// 	stoppedThreadCount++
		// 	autoScaledThreads = append(autoScaledThreads[:i], autoScaledThreads[i+1:]...)
		// 	continue
		// }
	}
	logger.Debug("deactivateThreads finished", slog.Int("stoppedThreadCount", stoppedThreadCount))
}
