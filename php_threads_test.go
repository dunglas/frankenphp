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

func TestStartAndStopTheMainThreadWithOneInactiveThread(t *testing.T) {
	logger = zap.NewNop()                // the logger needs to not be nil
	assert.NoError(t, initPHPThreads(1)) // reserve 1 thread

	assert.Len(t, phpThreads, 1)
	assert.Equal(t, 0, phpThreads[0].threadIndex)
	assert.False(t, phpThreads[0].isActive.Load())
	assert.Nil(t, phpThreads[0].worker)

	drainPHPThreads()
	assert.Nil(t, phpThreads)
}

// We'll start 100 threads and check that their hooks work correctly
func TestStartAndStop100PHPThreadsThatDoNothing(t *testing.T) {
	logger = zap.NewNop() // the logger needs to not be nil
	numThreads := 100
	readyThreads := atomic.Uint64{}
	finishedThreads := atomic.Uint64{}
	workingThreads := atomic.Uint64{}
	workWG := sync.WaitGroup{}
	workWG.Add(numThreads)

	assert.NoError(t, initPHPThreads(numThreads))

	for i := 0; i < numThreads; i++ {
		newThread := getInactivePHPThread()
		newThread.setActive(
			// onStartup  => before the thread is ready
			func(thread *phpThread) {
				if thread.threadIndex == newThread.threadIndex {
					readyThreads.Add(1)
				}
			},
			// beforeScriptExecution => we stop here immediately
			func(thread *phpThread) {
				if thread.threadIndex == newThread.threadIndex {
					workingThreads.Add(1)
				}
				workWG.Done()
				newThread.setInactive()
			},
			// afterScriptExecution => no script is executed, we shouldn't reach here
			func(thread *phpThread, exitStatus int) {
				panic("hook afterScriptExecution should not be called here")
			},
			// onShutdown => after the thread is done
			func(thread *phpThread) {
				if thread.threadIndex == newThread.threadIndex {
					finishedThreads.Add(1)
				}
			},
		)
	}

	workWG.Wait()
	drainPHPThreads()

	assert.Equal(t, numThreads, int(readyThreads.Load()))
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
	workWG := sync.WaitGroup{}
	workWG.Add(maxExecutions)

	assert.NoError(t, initPHPThreads(numThreads))

	for i := 0; i < numThreads; i++ {
		getInactivePHPThread().setActive(
			// onStartup => fake a request on startup (like a worker would do)
			func(thread *phpThread) {
				r, _ := http.NewRequest(http.MethodGet, "sleep.php", nil)
				r, _ = NewRequestWithContext(r, WithRequestDocumentRoot("/", false))
				assert.NoError(t, updateServerContext(thread, r, true, false))
				thread.mainRequest = r
				thread.scriptName = scriptPath
			},
			// beforeScriptExecution => execute the sleep.php script until we reach maxExecutions
			func(thread *phpThread) {
				executionMutex.Lock()
				if executionCount >= maxExecutions {
					executionMutex.Unlock()
					thread.setInactive()
					return
				}
				executionCount++
				workWG.Done()
				executionMutex.Unlock()
			},
			// afterScriptExecution => check the exit status of the script
			func(thread *phpThread, exitStatus int) {
				if int(exitStatus) != 0 {
					panic("script execution failed: " + scriptPath)
				}
			},
			// onShutdown => nothing to do here
			nil,
		)
	}

	workWG.Wait()
	drainPHPThreads()

	assert.Equal(t, maxExecutions, executionCount)
}

// TODO: Make this test more chaotic
func TestStart100ThreadsAndConvertThemToDifferentThreads10Times(t *testing.T) {
	logger = zap.NewNop() // the logger needs to not be nil
	numThreads := 100
	numConversions := 10
	startUpTypes := make([]atomic.Uint64, numConversions)
	workTypes := make([]atomic.Uint64, numConversions)
	shutdownTypes := make([]atomic.Uint64, numConversions)
	workWG := sync.WaitGroup{}

	assert.NoError(t, initPHPThreads(numThreads))

	for i := 0; i < numConversions; i++ {
		workWG.Add(numThreads)
		numberOfConversion := i
		for j := 0; j < numThreads; j++ {
			getInactivePHPThread().setActive(
				// onStartup  => before the thread is ready
				func(thread *phpThread) {
					startUpTypes[numberOfConversion].Add(1)
				},
				// beforeScriptExecution => while the thread is running
				func(thread *phpThread) {
					workTypes[numberOfConversion].Add(1)
					thread.setInactive()
					workWG.Done()
				},
				// afterScriptExecution => we don't execute a script
				nil,
				// onShutdown => after the thread is done
				func(thread *phpThread) {
					shutdownTypes[numberOfConversion].Add(1)
				},
			)
		}
		workWG.Wait()
	}

	drainPHPThreads()

	// each type of thread needs to have started, worked and stopped the same amount of times
	for i := 0; i < numConversions; i++ {
		assert.Equal(t, numThreads, int(startUpTypes[i].Load()))
		assert.Equal(t, numThreads, int(workTypes[i].Load()))
		assert.Equal(t, numThreads, int(shutdownTypes[i].Load()))
	}
}
