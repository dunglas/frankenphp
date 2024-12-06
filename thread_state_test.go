package frankenphp

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestYieldToEachOtherViaThreadStates(t *testing.T) {
	threadState := &stateHandler{currentState: stateBooting}

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
	threadState := &stateHandler{currentState: stateBooting}
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
	// 'set' should have yielded to the wait group
	assert.True(t, hasYielded)
}
