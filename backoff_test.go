package frankenphp

import (
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestExponentialBackoff_Reset(t *testing.T) {
	e := &exponentialBackoff{
		maxBackoff:             5 * time.Second,
		minBackoff:             500 * time.Millisecond,
		maxConsecutiveFailures: 3,
	}

	assert.False(t, e.recordFailure())
	assert.False(t, e.recordFailure())
	e.recordSuccess()

	e.mu.RLock()
	defer e.mu.RUnlock()
	assert.Equal(t, 0, e.failureCount, "expected failureCount to be reset to 0")
	assert.Equal(t, e.backoff, e.minBackoff, "expected backoff to be reset to minBackoff")
}

func TestExponentialBackoff_Trigger(t *testing.T) {
	e := &exponentialBackoff{
		maxBackoff:             500 * 3 * time.Millisecond,
		minBackoff:             500 * time.Millisecond,
		maxConsecutiveFailures: 3,
	}

	assert.False(t, e.recordFailure())
	assert.False(t, e.recordFailure())
	assert.True(t, e.recordFailure())

	e.mu.RLock()
	defer e.mu.RUnlock()
	assert.Equal(t, e.failureCount, e.maxConsecutiveFailures, "expected failureCount to be maxConsecutiveFailures")
	assert.Equal(t, e.backoff, e.maxBackoff, "expected backoff to be maxBackoff")
}
