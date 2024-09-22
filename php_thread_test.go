package frankenphp

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestInitializeTwoPhpThreadsCorrectly(t *testing.T) {
	initializePhpThreads(2)

	assert.Len(t, phpThreads, 2)
	assert.Equal(t, 0, getPHPThread(0).threadIndex)
	assert.Equal(t, 1, getPHPThread(1).threadIndex)
	assert.Nil(t, getPHPThread(0).mainRequest)
	assert.Nil(t, getPHPThread(0).workerRequest)
}