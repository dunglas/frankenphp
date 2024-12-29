package frankenphp

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestScalingStrategyShouldBeNoneOnLowMaxThreads(t *testing.T) {
	doneChan := make(chan struct{})
	initAutoScaling(1, 1, ScalingStrategyNormal)

	assert.IsType(t, scalingStrategyNone{}, activeScalingStrategy)

	close(doneChan)
}

func TestScalingStrategyShouldBeNormal(t *testing.T) {
	doneChan := make(chan struct{})
	initAutoScaling(1, 2, ScalingStrategyNormal)

	assert.IsType(t, scalingStrategyNormal{}, activeScalingStrategy)

	close(doneChan)
}

func TestScalingStrategyShouldBeNoneWhenExplicitlySetToNone(t *testing.T) {
	doneChan := make(chan struct{})
	initAutoScaling(1, 2, ScalingStrategyNone)

	assert.IsType(t, scalingStrategyNone{}, activeScalingStrategy)

	close(doneChan)
}

func TestScaleARegularThreadUpAndDown(t *testing.T) {
	assert.NoError(t, Init(
		WithNumThreads(1),
		WithMaxThreads(2, ScalingStrategyNormal),
		WithLogger(zap.NewNop()),
	))

	autoScaledThread := phpThreads[1]

	// scale up
	scaleRegularThreads()
	assert.Equal(t, stateReady, autoScaledThread.state.get())
	assert.IsType(t, &regularThread{}, autoScaledThread.handler)

	// on the first down-scale, the thread will be marked as inactive
	setLongWaitTime(autoScaledThread)
	downScaleThreads()
	assert.IsType(t, &inactiveThread{}, autoScaledThread.handler)

	// on the second down-scale, the thread will be removed
	autoScaledThread.state.waitFor(stateInactive)
	setLongWaitTime(autoScaledThread)
	downScaleThreads()
	assert.Equal(t, stateReserved, autoScaledThread.state.get())

	Shutdown()
}

func TestScaleAWorkerThreadUpAndDown(t *testing.T) {
	workerPath := testDataPath + "/transition-worker-1.php"
	assert.NoError(t, Init(
		WithNumThreads(2),
		WithMaxThreads(3, ScalingStrategyNormal),
		WithWorkers(workerPath, 1, map[string]string{}, []string{}),
		WithLogger(zap.NewNop()),
	))

	autoScaledThread := phpThreads[2]

	// scale up
	scaleWorkerThreads(workers[workerPath])
	assert.Equal(t, stateReady, autoScaledThread.state.get())

	// on the first down-scale, the thread will be marked as inactive
	setLongWaitTime(autoScaledThread)
	downScaleThreads()
	assert.IsType(t, &inactiveThread{}, autoScaledThread.handler)

	// on the second down-scale, the thread will be removed
	autoScaledThread.state.waitFor(stateInactive)
	setLongWaitTime(autoScaledThread)
	downScaleThreads()
	assert.Equal(t, stateReserved, autoScaledThread.state.get())

	Shutdown()
}

func setLongWaitTime(thread *phpThread) {
	thread.state.mu.Lock()
	thread.state.waitingSince = time.Now().Add(-time.Hour)
	thread.state.mu.Unlock()
}
