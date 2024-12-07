package frankenphp

import (
	"slices"
	"sync"
)

type stateID int

const (
	stateBooting stateID = iota
	stateInactive
	stateActive
	stateReady
	stateBusy
	stateShuttingDown
	stateDone
	stateRestarting
	stateDrain
	stateYielding
)

type threadState struct {
	currentState stateID
	mu           sync.RWMutex
	subscribers  []stateSubscriber
}

type stateSubscriber struct {
	states []stateID
	ch     chan struct{}
}

func newThreadState() *threadState {
	return &threadState{
		currentState: stateBooting,
		subscribers:  []stateSubscriber{},
		mu:           sync.RWMutex{},
	}
}

func (h *threadState) is(state stateID) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.currentState == state
}

func (h *threadState) get() stateID {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.currentState
}

func (h *threadState) set(nextState stateID) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.currentState = nextState

	if len(h.subscribers) == 0 {
		return
	}

	newSubscribers := []stateSubscriber{}
	// notify subscribers to the state change
	for _, sub := range h.subscribers {
		if !slices.Contains(sub.states, nextState) {
			newSubscribers = append(newSubscribers, sub)
			continue
		}
		close(sub.ch)
	}
	h.subscribers = newSubscribers
}

// block until the thread reaches a certain state
func (h *threadState) waitFor(states ...stateID) {
	h.mu.Lock()
	if slices.Contains(states, h.currentState) {
		h.mu.Unlock()
		return
	}
	sub := stateSubscriber{
		states: states,
		ch:     make(chan struct{}),
	}
	h.subscribers = append(h.subscribers, sub)
	h.mu.Unlock()
	<-sub.ch
}
