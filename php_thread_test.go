package frankenphp

import (
	"github.com/stretchr/testify/assert"
	"net/http"
	"testing"
)

func TestInitializeTwoPhpThreadsWithoutRequests(t *testing.T) {
	initializePHPThreads(2)

	assert.Len(t, phpThreads, 2)
	assert.Equal(t, 0, getPHPThread(0).threadIndex)
	assert.Equal(t, 1, getPHPThread(1).threadIndex)
	assert.Nil(t, getPHPThread(0).mainRequest)
	assert.Nil(t, getPHPThread(0).workerRequest)
}

func TestMainRequestIsActiveRequest(t *testing.T) {
	mainRequest := &http.Request{}
	initializePHPThreads(1)
	thread := getPHPThread(0)

	thread.setMainRequest(mainRequest)

	assert.Equal(t, mainRequest, thread.getActiveRequest())
}

func TestWorkerRequestIsActiveRequest(t *testing.T) {
	mainRequest := &http.Request{}
	workerRequest := &http.Request{}
	initializePHPThreads(1)
	thread := getPHPThread(0)

	thread.setMainRequest(mainRequest)
	thread.setWorkerRequest(workerRequest)

	assert.Equal(t, workerRequest, thread.getActiveRequest())
	assert.Equal(t, mainRequest, thread.getMainRequest())
}
