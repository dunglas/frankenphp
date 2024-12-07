package frankenphp

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestYieldToEachOtherViaThreadStates(t *testing.T) {
	threadState := &threadState{currentState: stateBooting}

	go func() {
		threadState.waitFor(stateInactive)
		assert.True(t, threadState.is(stateInactive))
		threadState.set(stateActive)
	}()

	threadState.set(stateInactive)
	threadState.waitFor(stateActive)
	assert.True(t, threadState.is(stateActive))
}
