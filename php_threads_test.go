package frankenphp

import (
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
