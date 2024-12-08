package frankenphp

import (
	"io"
	"math/rand/v2"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

var testDataPath, _ = filepath.Abs("./testdata")

func TestStartAndStopTheMainThreadWithOneInactiveThread(t *testing.T) {
	logger = zap.NewNop()                   // the logger needs to not be nil
	assert.NoError(t, initPHPThreads(1, 1)) // boot 1 thread

	assert.Len(t, phpThreads, 1)
	assert.Equal(t, 0, phpThreads[0].threadIndex)
	assert.True(t, phpThreads[0].state.is(stateInactive))

	drainPHPThreads()
	assert.Nil(t, phpThreads)
}

func TestTransitionRegularThreadToWorkerThread(t *testing.T) {
	logger = zap.NewNop()
	assert.NoError(t, initPHPThreads(1, 1))

	// transition to regular thread
	convertToRegularThread(phpThreads[0])
	assert.IsType(t, &regularThread{}, phpThreads[0].handler)

	// transition to worker thread
	worker := getDummyWorker("worker-transition-1.php")
	convertToWorkerThread(phpThreads[0], worker)
	assert.IsType(t, &workerThread{}, phpThreads[0].handler)
	assert.Len(t, worker.threads, 1)

	// transition back to inactive thread
	convertToInactiveThread(phpThreads[0])
	assert.IsType(t, &inactiveThread{}, phpThreads[0].handler)
	assert.Len(t, worker.threads, 0)

	drainPHPThreads()
	assert.Nil(t, phpThreads)
}

func TestTransitionAThreadBetween2DifferentWorkers(t *testing.T) {
	logger = zap.NewNop()
	assert.NoError(t, initPHPThreads(1, 1))
	firstWorker := getDummyWorker("worker-transition-1.php")
	secondWorker := getDummyWorker("worker-transition-2.php")

	// convert to first worker thread
	convertToWorkerThread(phpThreads[0], firstWorker)
	firstHandler := phpThreads[0].handler.(*workerThread)
	assert.Same(t, firstWorker, firstHandler.worker)
	assert.Len(t, firstWorker.threads, 1)
	assert.Len(t, secondWorker.threads, 0)

	// convert to second worker thread
	convertToWorkerThread(phpThreads[0], secondWorker)
	secondHandler := phpThreads[0].handler.(*workerThread)
	assert.Same(t, secondWorker, secondHandler.worker)
	assert.Len(t, firstWorker.threads, 0)
	assert.Len(t, secondWorker.threads, 1)

	drainPHPThreads()
	assert.Nil(t, phpThreads)
}

func TestTransitionThreadsWhileDoingRequests(t *testing.T) {
	numThreads := 10
	numRequestsPerThread := 100
	isRunning := atomic.Bool{}
	isRunning.Store(true)
	wg := sync.WaitGroup{}
	worker1Path := testDataPath + "/transition-worker-1.php"
	worker2Path := testDataPath + "/transition-worker-2.php"

	assert.NoError(t, Init(
		WithNumThreads(numThreads),
		WithWorkers(worker1Path, 1, map[string]string{"ENV1": "foo"}, []string{}),
		WithWorkers(worker2Path, 1, map[string]string{"ENV1": "foo"}, []string{}),
		WithLogger(zap.NewNop()),
	))

	// randomly transition threads between regular, inactive and 2 worker threads
	go func() {
		for {
			for i := 0; i < numThreads; i++ {
				switch rand.IntN(4) {
				case 0:
					convertToRegularThread(phpThreads[i])
				case 1:
					convertToWorkerThread(phpThreads[i], workers[worker1Path])
				case 2:
					convertToWorkerThread(phpThreads[i], workers[worker2Path])
				case 3:
					convertToInactiveThread(phpThreads[i])
				}
				time.Sleep(time.Millisecond)
				if !isRunning.Load() {
					return
				}
			}
		}
	}()

	// randomly do requests to the 3 endpoints
	wg.Add(numThreads)
	for i := 0; i < numThreads; i++ {
		go func(i int) {
			for j := 0; j < numRequestsPerThread; j++ {
				switch rand.IntN(3) {
				case 0:
					assertRequestBody(t, "http://localhost/transition-worker-1.php", "Hello from worker 1")
				case 1:
					assertRequestBody(t, "http://localhost/transition-worker-2.php", "Hello from worker 2")
				case 2:
					assertRequestBody(t, "http://localhost/transition-regular.php", "Hello from regular thread")
				}
			}
			wg.Done()
		}(i)
	}

	wg.Wait()
	isRunning.Store(false)
	Shutdown()
}

func getDummyWorker(fileName string) *worker {
	if workers == nil {
		workers = make(map[string]*worker)
	}
	worker, _ := newWorker(workerOpt{
		fileName: testDataPath + "/" + fileName,
		num:      1,
	})
	return worker
}

func assertRequestBody(t *testing.T, url string, expected string) {
	r := httptest.NewRequest("GET", url, nil)
	w := httptest.NewRecorder()

	req, err := NewRequestWithContext(r, WithRequestDocumentRoot(testDataPath, false))
	assert.NoError(t, err)
	err = ServeHTTP(w, req)
	assert.NoError(t, err)
	resp := w.Result()
	body, _ := io.ReadAll(resp.Body)
	assert.Equal(t, expected, string(body))
}
