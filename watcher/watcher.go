package watcher

// #cgo LDFLAGS: -lwatcher -lstdc++
// #cgo CFLAGS: -Wall -Werror
// #include <stdint.h>
// #include <stdlib.h>
// #include "watcher.h"
import "C"
import (
	"errors"
	"go.uber.org/zap"
	"runtime/cgo"
	"sync"
	"time"
	"unsafe"
)

type watcher struct {
	sessions  []unsafe.Pointer
	callback  func()
	watchOpts []WatchOpt
	trigger   chan struct{}
	stop      chan struct{}
}

// duration to wait before triggering a reload after a file change
const debounceDuration = 150 * time.Millisecond

var (
	// the currently active file watcher
	activeWatcher *watcher
	// after stopping the watcher we will wait for eventual reloads to finish
	reloadWaitGroup sync.WaitGroup
	logger          *zap.Logger
)

func InitWatcher(watchOpts []WatchOpt, callback func(), zapLogger *zap.Logger) error {
	if len(watchOpts) == 0 {
		return nil
	}
	if activeWatcher != nil {
		return errors.New("watcher is already running")
	}
	logger = zapLogger
	activeWatcher = &watcher{callback: callback}
	err := activeWatcher.startWatching(watchOpts)
	if err != nil {
		return err
	}
	reloadWaitGroup = sync.WaitGroup{}

	return nil
}

func DrainWatcher() {
	if activeWatcher == nil {
		return
	}
	logger.Info("stopping watcher...")
	activeWatcher.stopWatching()
	reloadWaitGroup.Wait()
	activeWatcher = nil
}

func (w *watcher) startWatching(watchOpts []WatchOpt) error {
	w.trigger = make(chan struct{})
	w.stop = make(chan struct{})
	w.sessions = make([]unsafe.Pointer, len(watchOpts))
	w.watchOpts = watchOpts
	for i, watchOpt := range w.watchOpts {
		watchOpt.trigger = w.trigger
		session, err := startSession(&watchOpt)
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

func startSession(watchOpt *WatchOpt) (unsafe.Pointer, error) {
	handle := cgo.NewHandle(watchOpt)
	cDir := C.CString(watchOpt.dir)
	defer C.free(unsafe.Pointer(cDir))
	watchSession := C.start_new_watcher(cDir, C.uintptr_t(handle))
	if watchSession != C.NULL {
		logger.Debug("watching", zap.String("dir", watchOpt.dir), zap.String("pattern", watchOpt.pattern))
		return watchSession, nil
	}
	logger.Error("couldn't start watching", zap.String("dir", watchOpt.dir))

	return nil, errors.New("couldn't start watching")
}

func stopSession(session unsafe.Pointer) {
	success := C.stop_watcher(session)
	if success == 1 {
		logger.Error("couldn't stop watching")
	}
}

//export go_handle_file_watcher_event
func go_handle_file_watcher_event(path *C.char, eventType C.int, pathType C.int, handle C.uintptr_t) {
	watchOpt := cgo.Handle(handle).Value().(*WatchOpt)
	if watchOpt.allowReload(C.GoString(path), int(eventType), int(pathType)) {
		watchOpt.trigger <- struct{}{}
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
