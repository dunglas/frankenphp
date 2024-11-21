package frankenphp

import (
	"sync"
	"time"
)

type exponentialBackoff struct {
	backoff                time.Duration
	failureCount           int
	mu                     sync.RWMutex
	maxBackoff             time.Duration
	minBackoff             time.Duration
	maxConsecutiveFailures int
}

func newExponentialBackoff(minBackoff time.Duration, maxBackoff time.Duration, maxConsecutiveFailures int) *exponentialBackoff {
	return &exponentialBackoff{
		backoff:                minBackoff,
		minBackoff:             minBackoff,
		maxBackoff:             maxBackoff,
		maxConsecutiveFailures: maxConsecutiveFailures,
	}
}

// recordSuccess resets the backoff and failureCount
func (e *exponentialBackoff) recordSuccess() {
	e.mu.Lock()
	e.failureCount = 0
	e.backoff = e.minBackoff
	e.mu.Unlock()
}

// recordFailure increments the failure count and increases the backoff, it returns true if maxConsecutiveFailures has been reached
func (e *exponentialBackoff) recordFailure() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.failureCount += 1
	e.backoff = min(e.backoff*2, e.maxBackoff)

	if e.failureCount >= e.maxConsecutiveFailures {
		return true
	}

	return false
}

// wait sleeps for the backoff duration if failureCount is non-zero.
// NOTE: this is not tested and should be kept 'obviously correct' (i.e., simple)
func (e *exponentialBackoff) wait() {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if e.failureCount == 0 {
		return
	}

	time.Sleep(e.backoff)
}
