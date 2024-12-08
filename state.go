package frankenphp

import (
	"slices"
	"sync"
)

type stateID int

const (
	// livecycle states of a thread
	stateReserved stateID = iota
	stateBooting
	stateInactive
	stateReady
	stateShuttingDown
	stateDone

	// states necessary for restarting workers
	stateRestarting
	stateYielding

	// states necessary for transitioning between different handlers
	stateTransitionRequested
	stateTransitionInProgress
	stateTransitionComplete
)

var stateNames = map[stateID]string{
	stateReserved:             "reserved",
	stateBooting:              "booting",
	stateInactive:             "inactive",
	stateReady:                "ready",
	stateShuttingDown:         "shutting down",
	stateDone:                 "done",
	stateRestarting:           "restarting",
	stateYielding:             "yielding",
	stateTransitionRequested:  "transition requested",
	stateTransitionInProgress: "transition in progress",
	stateTransitionComplete:   "transition complete",
}

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
		currentState: stateReserved,
		subscribers:  []stateSubscriber{},
		mu:           sync.RWMutex{},
	}
}

func (ts *threadState) is(state stateID) bool {
	ts.mu.RLock()
	defer ts.mu.RUnlock()
	return ts.currentState == state
}

func (ts *threadState) compareAndSwap(compareTo stateID, swapTo stateID) bool {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	if ts.currentState == compareTo {
		ts.currentState = swapTo
		return true
	}
	return false
}

func (ts *threadState) name() string {
	return stateNames[ts.get()]
}

func (ts *threadState) get() stateID {
	ts.mu.RLock()
	defer ts.mu.RUnlock()
	return ts.currentState
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
