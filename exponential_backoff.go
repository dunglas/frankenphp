package frankenphp

import (
	"sync"
	"time"
)

const maxBackoff = 1 * time.Second
const minBackoff = 100 * time.Millisecond
const maxConsecutiveFailures = 6

type exponentialBackoff struct {
	backoff      time.Duration
	failureCount int
	mu           sync.RWMutex
	upFunc       sync.Once
}

func newExponentialBackoff() *exponentialBackoff {
	return &exponentialBackoff{backoff: minBackoff}
}

func (e *exponentialBackoff) reset() {
	e.mu.Lock()
	e.upFunc = sync.Once{}
	wait := e.backoff * 2
	e.mu.Unlock()
	go func() {
		time.Sleep(wait)
		e.mu.Lock()
		defer e.mu.Unlock()
		e.upFunc.Do(func() {
			// if we come back to a stable state, reset the failure count
			if e.backoff == minBackoff {
				e.failureCount = 0
			}

			// earn back the backoff over time
			if e.failureCount > 0 {
				e.backoff = max(e.backoff/2, minBackoff)
			}
		})
	}()
}

func (e *exponentialBackoff) trigger(onMaxFailures func(failureCount int)) {
	e.mu.RLock()
	e.upFunc.Do(func() {
		if e.failureCount >= maxConsecutiveFailures {
			onMaxFailures(e.failureCount)
		}
		e.failureCount += 1
	})
	wait := e.backoff
	e.mu.RUnlock()
	time.Sleep(wait)
	e.mu.Lock()
	e.backoff = min(e.backoff*2, maxBackoff)
	e.mu.Unlock()
}
