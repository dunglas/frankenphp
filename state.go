package frankenphp

import (
	"slices"
	"sync"
)

type stateID uint8

const (
	// livecycle states of a thread
	stateReserved stateID = iota
	stateBooting
	stateShuttingDown
	stateDone

	// these states are safe to transition from at any time
	stateInactive
	stateReady

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
	ok := ts.currentState == state
	ts.mu.RUnlock()

	return ok
}

func (ts *threadState) compareAndSwap(compareTo stateID, swapTo stateID) bool {
	ts.mu.Lock()
	ok := ts.currentState == compareTo
	if ok {
		ts.currentState = swapTo
		ts.notifySubscribers(swapTo)
	}
	ts.mu.Unlock()

	return ok
}

func (ts *threadState) name() string {
	return stateNames[ts.get()]
}

func (ts *threadState) get() stateID {
	ts.mu.RLock()
	id := ts.currentState
	ts.mu.RUnlock()

	return id
}

func (ts *threadState) set(nextState stateID) {
	ts.mu.Lock()
	ts.currentState = nextState
	ts.notifySubscribers(nextState)
	ts.mu.Unlock()
}

func (ts *threadState) notifySubscribers(nextState stateID) {
	if len(ts.subscribers) == 0 {
		return
	}
	newSubscribers := []stateSubscriber{}
	// notify subscribers to the state change
	for _, sub := range ts.subscribers {
		if !slices.Contains(sub.states, nextState) {
			newSubscribers = append(newSubscribers, sub)
			continue
		}
		close(sub.ch)
	}
	ts.subscribers = newSubscribers
}

// block until the thread reaches a certain state
func (ts *threadState) waitFor(states ...stateID) {
	ts.mu.Lock()
	if slices.Contains(states, ts.currentState) {
		ts.mu.Unlock()
		return
	}
	sub := stateSubscriber{
		states: states,
		ch:     make(chan struct{}),
	}
	ts.subscribers = append(ts.subscribers, sub)
	ts.mu.Unlock()
	<-sub.ch
}

// safely request a state change from a different goroutine
func (ts *threadState) requestSafeStateChange(nextState stateID) bool {
	ts.mu.Lock()
	switch ts.currentState {
	// disallow state changes if shutting down or done
	case stateShuttingDown, stateDone, stateReserved:
		ts.mu.Unlock()
		return false
	// ready and inactive are safe states to transition from
	case stateReady, stateInactive:
		ts.currentState = nextState
		ts.notifySubscribers(nextState)
		ts.mu.Unlock()
		return true
	}
	ts.mu.Unlock()

	// wait for the state to change to a safe state
	ts.waitFor(stateReady, stateInactive, stateShuttingDown)
	return ts.requestSafeStateChange(nextState)
}
