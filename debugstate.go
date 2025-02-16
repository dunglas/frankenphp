package frankenphp

import (
	"net/http"
	"time"
)

// EXPERIMENTAL: ThreadDebugState prints the state of a single PHP thread - debugging purposes only
type ThreadDebugState struct {
	Index                      int
	Name                       string
	State                      string
	IsHandlingRequest          bool
	Path                       string
	InRequestSinceMilliseconds int64
	WaitingSinceMilliseconds   int64
}

// EXPERIMENTAL: FrankenPHPDebugState prints the state of all PHP threads - debugging purposes only
type FrankenPHPDebugState struct {
	ThreadDebugStates   []ThreadDebugState
	ReservedThreadCount int
}

// EXPERIMENTAL: DebugState prints the state of all PHP threads - debugging purposes only
func DebugState() FrankenPHPDebugState {
	fullState := FrankenPHPDebugState{
		ThreadDebugStates:   make([]ThreadDebugState, 0, len(phpThreads)),
		ReservedThreadCount: 0,
	}
	for _, thread := range phpThreads {
		if thread.state.is(stateReserved) {
			fullState.ReservedThreadCount++
			continue
		}
		fullState.ThreadDebugStates = append(fullState.ThreadDebugStates, threadDebugState(thread))
	}

	return fullState
}

// threadDebugState creates a small jsonable status message for debugging purposes
func threadDebugState(thread *phpThread) ThreadDebugState {
	debugState := ThreadDebugState{
		Index:                    thread.threadIndex,
		Name:                     thread.handler.name(),
		State:                    thread.state.name(),
		WaitingSinceMilliseconds: thread.state.waitTime(),
	}

	var r *http.Request
	if r = thread.getActiveRequestSafely(); r == nil {
		return debugState
	}

	fc := r.Context().Value(contextKey).(*FrankenPHPContext)
	if fc.originalRequest != nil {
		debugState.Path = fc.originalRequest.URL.Path
	} else {
		debugState.Path = r.URL.Path
	}

	if fc.responseWriter != nil {
		debugState.IsHandlingRequest = true
		debugState.InRequestSinceMilliseconds = time.Since(fc.startedAt).Milliseconds()
	}

	return debugState
}
