package frankenphp

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInitializeTwoPhpThreadsWithoutRequests(t *testing.T) {
	initPHPThreads(2)

	assert.Len(t, phpThreads, 2)
	assert.NotNil(t, phpThreads[0])
	assert.NotNil(t, phpThreads[1])
	assert.Nil(t, phpThreads[0].mainRequest)
	assert.Nil(t, phpThreads[0].workerRequest)
}

func TestMainRequestIsActiveRequest(t *testing.T) {
	mainRequest := &http.Request{}
	initPHPThreads(1)
	thread := phpThreads[0]

	thread.mainRequest = mainRequest

	assert.Equal(t, mainRequest, thread.getActiveRequest())
}

func TestWorkerRequestIsActiveRequest(t *testing.T) {
	mainRequest := &http.Request{}
	workerRequest := &http.Request{}
	initPHPThreads(1)
	thread := phpThreads[0]

	thread.mainRequest = mainRequest
	thread.workerRequest = workerRequest

	assert.Equal(t, workerRequest, thread.getActiveRequest())
}
