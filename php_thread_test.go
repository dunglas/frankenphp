package frankenphp

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMainRequestIsActiveRequest(t *testing.T) {
	mainRequest := &http.Request{}
	thread := phpThread{}

	thread.mainRequest = mainRequest

	assert.Equal(t, mainRequest, thread.getActiveRequest())
}

func TestWorkerRequestIsActiveRequest(t *testing.T) {
	mainRequest := &http.Request{}
	workerRequest := &http.Request{}
	thread := phpThread{}

	thread.mainRequest = mainRequest
	thread.workerRequest = workerRequest

	assert.Equal(t, workerRequest, thread.getActiveRequest())
}
