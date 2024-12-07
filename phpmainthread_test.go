package frankenphp

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestStartAndStopTheMainThreadWithOneInactiveThread(t *testing.T) {
	logger = zap.NewNop()                // the logger needs to not be nil
	assert.NoError(t, initPHPThreads(1)) // reserve 1 thread

	assert.Len(t, phpThreads, 1)
	assert.Equal(t, 0, phpThreads[0].threadIndex)
	assert.True(t, phpThreads[0].state.is(stateInactive))

	drainPHPThreads()
	assert.Nil(t, phpThreads)
}

func TestTransition2RegularThreadsToWorkerThreadsAndBack(t *testing.T) {
	numThreads := 2
	logger, _ = zap.NewDevelopment()
	assert.NoError(t, initPHPThreads(numThreads))

	// transition to worker thread
	for i := 0; i < numThreads; i++ {
		convertToRegularThread(phpThreads[i])
		assert.IsType(t, &regularThread{}, phpThreads[i].handler)
	}

	// transition to worker thread
	worker := getDummyWorker()
	for i := 0; i < numThreads; i++ {
		convertToWorkerThread(phpThreads[i], worker)
		assert.IsType(t, &workerThread{}, phpThreads[i].handler)
	}
	assert.Len(t, worker.threads, numThreads)

	// transition back to regular thread
	for i := 0; i < numThreads; i++ {
		convertToRegularThread(phpThreads[i])
		assert.IsType(t, &regularThread{}, phpThreads[i].handler)
	}
	assert.Len(t, worker.threads, 0)

	drainPHPThreads()
	assert.Nil(t, phpThreads)
}

func TestTransitionAThreadBetween2DifferentWorkers(t *testing.T) {
	logger, _ = zap.NewDevelopment()
	assert.NoError(t, initPHPThreads(1))

	// convert to first worker thread
	firstWorker := getDummyWorker()
	convertToWorkerThread(phpThreads[0], firstWorker)
	firstHandler := phpThreads[0].handler.(*workerThread)
	assert.Same(t, firstWorker, firstHandler.worker)

	// convert to second worker thread
	secondWorker := getDummyWorker()
	convertToWorkerThread(phpThreads[0], secondWorker)
	secondHandler := phpThreads[0].handler.(*workerThread)
	assert.Same(t, secondWorker, secondHandler.worker)

	drainPHPThreads()
	assert.Nil(t, phpThreads)
}

func getDummyWorker() *worker {
	path, _ := filepath.Abs("./testdata/index.php")
	return &worker{fileName: path}
}
