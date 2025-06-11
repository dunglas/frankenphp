package frankenphp

import (
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestScaleARegularThreadUpAndDown(t *testing.T) {
	assert.NoError(t, Init(
		WithNumThreads(1),
		WithMaxThreads(2),
		WithLogger(slog.New(slog.NewTextHandler(io.Discard, nil))),
	))

	autoScaledThread := phpThreads[1]

	// scale up
	scaleRegularThread()
	assert.Equal(t, stateReady, autoScaledThread.state.get())
	assert.IsType(t, &regularThread{}, autoScaledThread.handler)

	// on down-scale, the thread will be marked as inactive
	setLongWaitTime(autoScaledThread)
	deactivateThreads()
	assert.IsType(t, &inactiveThread{}, autoScaledThread.handler)

	Shutdown()
}

func TestScaleAWorkerThreadUpAndDown(t *testing.T) {
	workerName := "worker1"
	workerPath := testDataPath + "/transition-worker-1.php"
	assert.NoError(t, Init(
		WithNumThreads(2),
		WithMaxThreads(3),
		WithWorkers(workerName, workerPath, 1, map[string]string{}, []string{}),
		WithLogger(slog.New(slog.NewTextHandler(io.Discard, nil))),
	))

	autoScaledThread := phpThreads[2]

	// scale up
	scaleWorkerThread(getWorkerByPath(workerPath))
	assert.Equal(t, stateReady, autoScaledThread.state.get())

	// on down-scale, the thread will be marked as inactive
	setLongWaitTime(autoScaledThread)
	deactivateThreads()
	assert.IsType(t, &inactiveThread{}, autoScaledThread.handler)

	Shutdown()
}

func setLongWaitTime(thread *phpThread) {
	thread.state.mu.Lock()
	thread.state.waitingSince = time.Now().Add(-time.Hour)
	thread.state.mu.Unlock()
}
