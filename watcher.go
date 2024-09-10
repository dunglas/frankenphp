package frankenphp

// #cgo LDFLAGS: -lwatcher-c-0.11.0
// #cgo CFLAGS: -Wall -Werror
// #include <stdint.h>
// #include <stdlib.h>
// #include "watcher.h"
import "C"
import (

	"go.uber.org/zap"
	"sync"
	"time"
	"runtime/cgo"
	"unsafe"
	"errors"
)

type watcher struct {
	sessions   []unsafe.Pointer
	workerOpts []workerOpt
	watchOpts []watchOpt
}

// duration to wait before reloading workers after a file change
const debounceDuration = 150 * time.Millisecond

var (
	// the currently active file watcher
	activeWatcher *watcher
	// when stopping the watcher we need to wait for reloading to finish
	reloadWaitGroup sync.WaitGroup
	triggerWatcher  chan struct{}
	stopWatcher    chan struct{}
)

func initWatcher(watchOpts []watchOpt, workerOpts []workerOpt) error {
	if len(watchOpts) == 0 {
		return nil
	}
	activeWatcher = &watcher{workerOpts: workerOpts}
	err := activeWatcher.startWatching(watchOpts)
	if err != nil {
		return err
	}
	reloadWaitGroup = sync.WaitGroup{}

	return nil
}

func drainWatcher() {
	if activeWatcher == nil {
		return
	}
	logger.Info("stopping watcher...")
	activeWatcher.stopWatching()
	reloadWaitGroup.Wait()
	activeWatcher = nil
}

func startWatchOpt(watchOpt *watchOpt) (unsafe.Pointer, error) {
	handle := cgo.NewHandle(watchOpt)
	cPathTranslated := (*C.char)(C.CString(watchOpt.dirs[0]))
    watchSession := C.start_new_watcher(cPathTranslated, C.uintptr_t(handle))
    if(watchSession == C.NULL){
    	logger.Error("couldn't start watching", zap.Strings("dirs", watchOpt.dirs))
    	return nil, errors.New("couldn't start watching")
    }
	return watchSession, nil
}

func stopWatchSession(session unsafe.Pointer){
	success := C.stop_watcher(session)
	if(success == 0){
		logger.Error("couldn't stop watching")
	}
}

//export go_handle_event
func go_handle_event(path *C.char, eventType C.int, pathType C.int, handle C.uintptr_t) {
	watchOpt := cgo.Handle(handle).Value().(*watchOpt)
	if watchOpt.allowReload(C.GoString(path), int(eventType), int(pathType)) {
		logger.Debug("valid file change detected", zap.String("path",  C.GoString(path)))
		triggerWatcher <- struct{}{}
	}
}

func (w *watcher) startWatching(watchOpts []watchOpt) error {
	w.sessions = make([]unsafe.Pointer, len(watchOpts))
	w.watchOpts = watchOpts
	triggerWatcher = make(chan struct{})
	stopWatcher = make(chan struct{})
	for i, watchOpt := range w.watchOpts {
		session, err := startWatchOpt(&watchOpt)
		if err != nil {
			logger.Error("unable to watch dirs", zap.Strings("dirs", watchOpt.dirs))
			return err
		}
		w.sessions[i] = session
	}
	go listenForFileEvents()
	return nil
}

func (w *watcher) stopWatching() {
	close(stopWatcher)
	for _, session := range w.sessions {
		stopWatchSession(session)
	}
}

func listenForFileEvents() {
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
			scheduleWorkerReload()
		}
	}
}

func scheduleWorkerReload() {
	logger.Info("filesystem change detected, restarting workers...")
	reloadWaitGroup.Add(1)
	restartWorkers(activeWatcher.workerOpts)
	reloadWaitGroup.Done()
}
