package frankenphp

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func ATestStartAndStopTheMainThread(t *testing.T) {
	logger = zap.NewNop()
	initPHPThreads(1) // reserve 1 thread

	assert.Equal(t, 1, len(phpThreads))
	assert.Equal(t, 0, phpThreads[0].threadIndex)
    assert.False(t, phpThreads[0].isActive)
    assert.False(t, phpThreads[0].isReady)
    assert.Nil(t, phpThreads[0].worker)

	drainPHPThreads()
	assert.Nil(t, phpThreads)
}

func ATestStartAndStopARegularThread(t *testing.T) {
	logger = zap.NewNop() // the logger needs to not be nil
	initPHPThreads(1)     // reserve 1 thread

	startNewPHPThread()
	threadsReadyWG.Wait()

	assert.Equal(t, 1, len(phpThreads))
	assert.True(t, phpThreads[0].isActive)
	assert.True(t, phpThreads[0].isReady)
	assert.Nil(t, phpThreads[0].worker)

	drainPHPThreads()
	assert.Nil(t, phpThreads)
}

func ATestStartAndStopAWorkerThread(t *testing.T) {
	logger = zap.NewNop() // the logger needs to not be nil
    initPHPThreads(1)     // reserve 1 thread

	initWorkers([]workerOpt{workerOpt {
	    fileName: "testdata/worker.php",
	    num:      1,
	    env:      make(map[string]string, 0),
	    watch:    make([]string, 0),
	}})
	threadsReadyWG.Wait()

	assert.Equal(t, 1, len(phpThreads))
	assert.True(t, phpThreads[0].isActive)
	assert.True(t, phpThreads[0].isReady)
	assert.NotNil(t, phpThreads[0].worker)

	drainPHPThreads()
	assert.Nil(t, phpThreads)
}

