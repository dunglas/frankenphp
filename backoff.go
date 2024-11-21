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
	e.failureCount += 1
	if e.backoff < e.minBackoff {
		e.backoff = e.minBackoff
	}

	e.backoff = min(e.backoff*2, e.maxBackoff)

	e.mu.Unlock()
	return e.failureCount >= e.maxConsecutiveFailures
}

// wait sleeps for the backoff duration if failureCount is non-zero.
// NOTE: this is not tested and should be kept 'obviously correct' (i.e., simple)
func (e *exponentialBackoff) wait() {
	e.mu.RLock()
	if e.failureCount == 0 {
		e.mu.RUnlock()

		return
	}
	e.mu.RUnlock()

	time.Sleep(e.backoff)
}
