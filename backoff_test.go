package frankenphp

import (
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestExponentialBackoff_Reset(t *testing.T) {
	e := newExponentialBackoff(500*time.Millisecond, 5*time.Second, 3)

	assert.False(t, e.recordFailure())
	assert.False(t, e.recordFailure())
	e.recordSuccess()

	e.mu.RLock()
	defer e.mu.RUnlock()
	if e.failureCount != 0 {
		t.Errorf("expected failureCount to be reset to 0, got %d", e.failureCount)
	}
	if e.backoff != e.minBackoff {
		t.Errorf("expected backoff to be reset to minBackoff, got %v", e.backoff)
	}
}

func TestExponentialBackoff_Trigger(t *testing.T) {
	e := newExponentialBackoff(500*time.Millisecond, 500*3*time.Millisecond, 3)

	assert.False(t, e.recordFailure())
	assert.False(t, e.recordFailure())
	assert.True(t, e.recordFailure())

	e.mu.RLock()
	defer e.mu.RUnlock()
	if e.failureCount != e.maxConsecutiveFailures {
		t.Errorf("expected failureCount to be %d, got %d", e.maxConsecutiveFailures, e.failureCount)
	}
	if e.backoff != e.maxBackoff {
		t.Errorf("expected backoff to be maxBackoff, got %v", e.backoff)
	}
}
