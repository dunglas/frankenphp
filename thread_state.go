package frankenphp

import (
	"slices"
	"sync"
)

type threadState int

const (
	stateBooting threadState = iota
	stateInactive
	stateActive
	stateReady
	stateBusy
	stateShuttingDown
	stateDone
	stateRestarting
)

type threadStateHandler struct {
	currentState threadState
	mu           sync.RWMutex
	subscribers  []stateSubscriber
}


type stateSubscriber struct {
	states   []threadState
	ch       chan struct{}
	yieldFor *sync.WaitGroup
}

func (h *threadStateHandler) is(state threadState) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.currentState == state
}

func (h *threadStateHandler) get(state threadState) threadState {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.currentState
}

func (h *threadStateHandler) set(nextState threadState) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.currentState == nextState {
		// TODO: do we return here or inform subscribers?
		// TODO: should we ever reach here?
		return
	}

	h.currentState = nextState

	if len(h.subscribers) == 0 {
		return
	}

	newSubscribers := []stateSubscriber{}
	// TODO: do we even need multiple subscribers?
	// notify subscribers to the state change
	for _, sub := range h.subscribers {
		if !slices.Contains(sub.states, nextState) {
			newSubscribers = append(newSubscribers, sub)
			continue
		}
		close(sub.ch)
		// yield for the subscriber
		if sub.yieldFor != nil {
			defer sub.yieldFor.Wait()
		}
	}
	h.subscribers = newSubscribers
}

// wait for the thread to reach a certain state
func (h *threadStateHandler) waitFor(states ...threadState) {
	h.waitForStates(states, nil)
}

// make the thread yield to a WaitGroup once it reaches the state
// this makes sure all threads are in sync both ways
func (h *threadStateHandler) waitForAndYield(yieldFor *sync.WaitGroup, states ...threadState) {
	h.waitForStates(states, yieldFor)
}

// subscribe to a state and wait until the thread reaches it
func (h *threadStateHandler) waitForStates(states []threadState, yieldFor *sync.WaitGroup) {
	h.mu.Lock()
	if slices.Contains(states, h.currentState) {
		h.mu.Unlock()
		return
	}
	sub := stateSubscriber{
		states:   states,
		ch:       make(chan struct{}),
		yieldFor: yieldFor,
	}
	h.subscribers = append(h.subscribers, sub)
	h.mu.Unlock()
	<-sub.ch
}
