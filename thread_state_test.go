package frankenphp

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestYieldToEachOtherViaThreadStates(t *testing.T) {
	threadState := &threadStateHandler{currentState: stateBooting}

	go func() {
		threadState.waitFor(stateInactive)
		assert.True(t, threadState.is(stateInactive))
		threadState.set(stateActive)
	}()

	threadState.set(stateInactive)
	threadState.waitFor(stateActive)
	assert.True(t, threadState.is(stateActive))
}

func TestYieldToAWaitGroupPassedByThreadState(t *testing.T) {
	logger, _ = zap.NewDevelopment()
	threadState := &threadStateHandler{currentState: stateBooting}
	hasYielded := false
	wg := sync.WaitGroup{}
	wg.Add(1)

	go func() {
		threadState.set(stateInactive)
		threadState.waitForAndYield(&wg, stateActive)
		hasYielded = true
		wg.Done()
	}()

	threadState.waitFor(stateInactive)
	threadState.set(stateActive)
	// the state should be 'ready' since we are also yielding to the WaitGroup
	assert.True(t, hasYielded)
}
