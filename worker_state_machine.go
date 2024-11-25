package frankenphp

import (
	"net/http"
	"sync"
	"strconv"

	"golang.uber.org/zap"
)

type workerStateMachine struct {
	state  *threadStateHandler
	thread *phpThread
	worker *worker
	isDone   bool
}


func (w *workerStateMachine) isDone() threadState {
	return w.isDone
}

func (w *workerStateMachine) handleState(nextState threadState) {
	previousState := w.state.get()

	switch previousState {
	case stateBooting:
		switch nextState {
		case stateInactive:
			w.state.set(stateInactive)
			// waiting for external signal to start
			w.state.waitFor(stateReady, stateShuttingDown)
			return
		}
	case stateInactive:
		switch nextState {
		case stateReady:
			w.state.set(stateReady)
			beforeScript(w.thread)
			return
		case stateShuttingDown:
			w.shutdown()
			return
		}
	case stateReady:
		switch nextState {
        case stateBusy:
            w.state.set(stateBusy)
            return
        case stateShuttingDown:
            w.shutdown()
            return
        }
	case stateBusy:
		afterScript(w.thread, w.worker)
		switch nextState {
		case stateReady:
			w.state.set(stateReady)
			beforeScript(w.thread, w.worker)
			return
		case stateShuttingDown:
			w.shutdown()
			return
		case stateRestarting:
			w.state.set(stateRestarting)
			return
		}
	case stateShuttingDown:
		switch nextState {
		case stateDone:
			w.thread.Unpin()
			w.state.set(stateDone)
			return
		case stateRestarting:
			w.state.set(stateRestarting)
			return
		}
	case stateDone:
		panic("Worker is done")
	case stateRestarting:
		switch nextState {
		case stateReady:
			// wait for external ready signal
			w.state.waitFor(stateReady)
			return
		case stateShuttingDown:
			w.shutdown()
			return
		}
	}

	panic("Invalid state transition from", zap.Int("from", int(previousState)), zap.Int("to", int(nextState)))
}

func (w *workerStateMachine) shutdown() {
	w.thread.scriptName = ""
	workerStateMachine.done = true
	w.thread.state.set(stateShuttingDown)
}

func beforeScript(thread *phpThread, worker *worker) {
	thread.worker = worker
	// if we are restarting due to file watching, set the state back to ready
	if thread.state.is(stateRestarting) {
		thread.state.set(stateReady)
	}

	thread.backoff.reset()
	metrics.StartWorker(worker.fileName)

	// Create a dummy request to set up the worker
	r, err := http.NewRequest(http.MethodGet, filepath.Base(worker.fileName), nil)
	if err != nil {
		panic(err)
	}

	r, err = NewRequestWithContext(
		r,
		WithRequestDocumentRoot(filepath.Dir(worker.fileName), false),
		WithRequestPreparedEnv(worker.env),
	)
	if err != nil {
		panic(err)
	}

	if err := updateServerContext(thread, r, true, false); err != nil {
		panic(err)
	}

	thread.mainRequest = r
	if c := logger.Check(zapcore.DebugLevel, "starting"); c != nil {
		c.Write(zap.String("worker", worker.fileName), zap.Int("thread", thread.threadIndex))
	}
}

func (worker *worker) afterScript(thread *phpThread, exitStatus int) {
	fc := thread.mainRequest.Context().Value(contextKey).(*FrankenPHPContext)
	fc.exitStatus = exitStatus

	defer func() {
		maybeCloseContext(fc)
		thread.mainRequest = nil
	}()

	// on exit status 0 we just run the worker script again
	if fc.exitStatus == 0 {
		// TODO: make the max restart configurable
		metrics.StopWorker(worker.fileName, StopReasonRestart)

		if c := logger.Check(zapcore.DebugLevel, "restarting"); c != nil {
			c.Write(zap.String("worker", worker.fileName))
		}
		return
	}

	// on exit status 1 we apply an exponential backoff when restarting
	metrics.StopWorker(worker.fileName, StopReasonCrash)
	thread.backoff.trigger(func(failureCount int) {
		// if we end up here, the worker has not been up for backoff*2
		// this is probably due to a syntax error or another fatal error
		if !watcherIsEnabled {
			panic(fmt.Errorf("workers %q: too many consecutive failures", worker.fileName))
		}
		logger.Warn("many consecutive worker failures", zap.String("worker", worker.fileName), zap.Int("failures", failureCount))
	})
}
