//go:build !nowatcher

package watcher

// #cgo LDFLAGS: -lwatcher-c -lstdc++
// #include <stdint.h>
// #include <stdlib.h>
// #include "watcher.h"
import "C"
import (
	"errors"
	"runtime/cgo"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"go.uber.org/zap"
)

type watcher struct {
	sessions []C.uintptr_t
	callback func()
	trigger  chan struct{}
	stop     chan struct{}
}

// duration to wait before triggering a reload after a file change
const debounceDuration = 150 * time.Millisecond

// times to retry watching if the watcher was closed prematurely
const maxFailureCount = 5
const failureResetDuration = 5 * time.Second

var failureMu = sync.Mutex{}
var watcherIsActive = atomic.Bool{}

var (
	// the currently active file watcher
	activeWatcher *watcher
	// after stopping the watcher we will wait for eventual reloads to finish
	reloadWaitGroup sync.WaitGroup
	// we are passing the logger from the main package to the watcher
	logger                *zap.Logger
	AlreadyStartedError   = errors.New("the watcher is already running")
	UnableToStartWatching = errors.New("unable to start the watcher")
)

func InitWatcher(filePatterns []string, callback func(), zapLogger *zap.Logger) error {
	if len(filePatterns) == 0 {
		return nil
	}
	if watcherIsActive.Load() {
		return AlreadyStartedError
	}
	watcherIsActive.Store(true)
	logger = zapLogger
	activeWatcher = &watcher{callback: callback}
	err := activeWatcher.startWatching(filePatterns)
	if err != nil {
		return err
	}
	reloadWaitGroup = sync.WaitGroup{}

	return nil
}

func DrainWatcher() {
	if !watcherIsActive.Load() {
		return
	}
	watcherIsActive.Store(false)
	logger.Debug("stopping watcher")
	activeWatcher.stopWatching()
	reloadWaitGroup.Wait()
	activeWatcher = nil
}

// TODO: how to test this?
func retryWatching(watchPattern *watchPattern) {
	failureMu.Lock()
	defer failureMu.Unlock()
	if watchPattern.failureCount >= maxFailureCount {
		return
	}
	logger.Info("watcher was closed prematurely, retrying...", zap.String("dir", watchPattern.dir))

	watchPattern.failureCount++
	session, err := startSession(watchPattern)
	if err != nil {
		activeWatcher.sessions = append(activeWatcher.sessions, session)
	}

	// reset the failure-count if the watcher hasn't reached max failures after 5 seconds
	go func() {
		time.Sleep(failureResetDuration * time.Second)
		failureMu.Lock()
		if watchPattern.failureCount < maxFailureCount {
			watchPattern.failureCount = 0
		}
		failureMu.Unlock()
	}()
}

func (w *watcher) startWatching(filePatterns []string) error {
	w.trigger = make(chan struct{})
	w.stop = make(chan struct{})
	w.sessions = make([]C.uintptr_t, len(filePatterns))
	watchPatterns, err := parseFilePatterns(filePatterns)
	if err != nil {
		return err
	}
	for i, watchPattern := range watchPatterns {
		watchPattern.trigger = w.trigger
		session, err := startSession(watchPattern)
		if err != nil {
			return err
		}
		w.sessions[i] = session
	}
	go listenForFileEvents(w.trigger, w.stop)
	return nil
}

func (w *watcher) stopWatching() {
	close(w.stop)
	for _, session := range w.sessions {
		stopSession(session)
	}
}

func startSession(w *watchPattern) (C.uintptr_t, error) {
	handle := cgo.NewHandle(w)
	cDir := C.CString(w.dir)
	defer C.free(unsafe.Pointer(cDir))
	watchSession := C.start_new_watcher(cDir, C.uintptr_t(handle))
	if watchSession != 0 {
		logger.Debug("watching", zap.String("dir", w.dir), zap.Strings("patterns", w.patterns))
		return watchSession, nil
	}
	logger.Error("couldn't start watching", zap.String("dir", w.dir))

	return watchSession, UnableToStartWatching
}

func stopSession(session C.uintptr_t) {
	success := C.stop_watcher(session)
	if success == 0 {
		logger.Warn("couldn't close the watcher")
	}
}

//export go_handle_file_watcher_event
func go_handle_file_watcher_event(path *C.char, eventType C.int, pathType C.int, handle C.uintptr_t) {
	watchPattern := cgo.Handle(handle).Value().(*watchPattern)
	goPath := C.GoString(path)

	if watchPattern.allowReload(goPath, int(eventType), int(pathType)) {
		watchPattern.trigger <- struct{}{}
	}

	// If the watcher prematurely sends the die@ event, retry watching
	if pathType == 4 && strings.HasPrefix(goPath, "e/self/die@") && watcherIsActive.Load() {
		retryWatching(watchPattern)
	}
}

func listenForFileEvents(triggerWatcher chan struct{}, stopWatcher chan struct{}) {
	timer := time.NewTimer(debounceDuration)
	timer.Stop()
	defer timer.Stop()
	for {
		select {
		case <-stopWatcher:
			break
		case <-triggerWatcher:
			timer.Reset(debounceDuration)
		case <-timer.C:
			timer.Stop()
			scheduleReload()
		}
	}
}

func scheduleReload() {
	logger.Info("filesystem change detected")
	reloadWaitGroup.Add(1)
	activeWatcher.callback()
	reloadWaitGroup.Done()
}
