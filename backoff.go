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

func (e *exponentialBackoff) recordSuccess() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.failureCount = 0
	e.backoff = e.minBackoff
}

func (e *exponentialBackoff) recordFailure() bool {
	doTrigger := false
	e.mu.Lock()
	defer e.mu.Unlock()
	e.failureCount += 1
	e.backoff = min(e.backoff*2, e.maxBackoff)

	if e.failureCount >= e.maxConsecutiveFailures {
		doTrigger = true
	}

	return doTrigger
}

func (e *exponentialBackoff) wait() {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if e.failureCount == 0 {
		return
	}

	time.Sleep(e.backoff)
}
