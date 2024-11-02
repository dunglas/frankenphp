package frankenphp

import (
	"net/http"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestStartAndStopTheMainThread(t *testing.T) {
	initPHPThreads(1) // reserve 1 thread

	assert.Equal(t, 1, len(phpThreads))
	assert.Equal(t, 0, phpThreads[0].threadIndex)
	assert.False(t, phpThreads[0].isActive)
	assert.Nil(t, phpThreads[0].worker)

	drainPHPThreads()
	assert.Nil(t, phpThreads)
}

// We'll start 100 threads and check that their hooks work correctly
// onStartup  => before the thread is ready
// onWork     => while the thread is working
// onShutdown => after the thread is done
func TestStartAndStop100PHPThreadsThatDoNothing(t *testing.T) {
	numThreads := 100
	readyThreads := atomic.Uint64{}
	finishedThreads := atomic.Uint64{}
	workingThreads := atomic.Uint64{}
	initPHPThreads(numThreads)

	for i := 0; i < numThreads; i++ {
		newThread := getInactivePHPThread()
		newThread.onStartup = func(thread *phpThread) {
			if thread.threadIndex == newThread.threadIndex {
				readyThreads.Add(1)
			}
		}
		newThread.onWork = func(thread *phpThread) bool {
			if thread.threadIndex == newThread.threadIndex {
				workingThreads.Add(1)
			}
			return false // stop immediately
		}
		newThread.onShutdown = func(thread *phpThread) {
			if thread.threadIndex == newThread.threadIndex {
				finishedThreads.Add(1)
			}
		}
		newThread.run()
	}

	threadsReadyWG.Wait()

	assert.Equal(t, numThreads, int(readyThreads.Load()))

	drainPHPThreads()

	assert.Equal(t, numThreads, int(workingThreads.Load()))
	assert.Equal(t, numThreads, int(finishedThreads.Load()))
}

// This test calls sleep() 10.000 times for 1ms in 100 PHP threads.
func TestSleep10000TimesIn100Threads(t *testing.T) {
	logger = zap.NewNop() // the logger needs to not be nil
	numThreads := 100
	maxExecutions := 10000
	executionMutex := sync.Mutex{}
	executionCount := 0
	scriptPath, _ := filepath.Abs("./testdata/sleep.php")
	initPHPThreads(numThreads)

	for i := 0; i < numThreads; i++ {
		newThread := getInactivePHPThread()

		// fake a request on startup (like a worker would do)
		newThread.onStartup = func(thread *phpThread) {
			r, _ := http.NewRequest(http.MethodGet, "sleep.php", nil)
			r, _ = NewRequestWithContext(r, WithRequestDocumentRoot("/", false))
			assert.NoError(t, updateServerContext(r, true, false))
			thread.mainRequest = r
		}

		// execute the sleep.php script until we reach maxExecutions
		newThread.onWork = func(thread *phpThread) bool {
			executionMutex.Lock()
			if executionCount >= maxExecutions {
				executionMutex.Unlock()
				return false
			}
			executionCount++
			executionMutex.Unlock()
			if int(executeScriptCGI(scriptPath)) != 0 {
				return false
			}

			return true
		}
		newThread.run()
	}

	drainPHPThreads()

	assert.Equal(t, maxExecutions, executionCount)
}
