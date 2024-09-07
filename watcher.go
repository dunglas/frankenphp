package frankenphp

import (
	fswatch "github.com/dunglas/go-fswatch"
	"go.uber.org/zap"
	"sync"
	"time"
)

type watcher struct {
	sessions   []*fswatch.Session
	watchOpts  []*watchOpt
	workerOpts []workerOpt
	trigger	   chan struct{}
	stop       chan struct{}
}

// duration to wait before reloading workers after a file change
const debounceDuration = 100 * time.Millisecond

var (
	// the currently active file watcher
	activeWatcher *watcher
	// when stopping the watcher we need to wait for reloading to finish
	reloadWaitGroup sync.WaitGroup
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

func (w *watcher) startWatching(watchOpts []watchOpt) error {
	w.sessions = make([]*fswatch.Session, len(watchOpts))
	w.watchOpts = make([]*watchOpt, len(watchOpts))
	w.trigger = make(chan struct{})
	w.stop = make(chan struct{})
	for i, watchOpt := range watchOpts {
		session, err := createSession(&watchOpt, w.trigger)
		if err != nil {
			logger.Error("unable to watch dirs", zap.Strings("dirs", watchOpt.dirs))
			return err
		}
		w.watchOpts[i] = &watchOpt
		w.sessions[i] = session
		go startSession(session)
	}
	go listenForFileEvents(w.trigger, w.stop)
	return nil
}

func (w *watcher) stopWatching() {
	close(w.stop)
	for i, session := range w.sessions {
		w.watchOpts[i].isActive = false
		stopSession(session)
	}
}

func createSession(watchOpt *watchOpt, triggerWatcher chan struct{}) (*fswatch.Session, error) {
	opts := []fswatch.Option{
		fswatch.WithRecursive(watchOpt.isRecursive),
		fswatch.WithFollowSymlinks(watchOpt.followSymlinks),
		fswatch.WithEventTypeFilters(watchOpt.eventTypes),
		fswatch.WithLatency(watchOpt.latency),
		fswatch.WithMonitorType((fswatch.MonitorType)(watchOpt.monitorType)),
		fswatch.WithFilters(watchOpt.filters),
	}
	handleFileEvent := registerEventHandler(watchOpt, triggerWatcher)
	logger.Debug("starting watcher session", zap.Strings("dirs", watchOpt.dirs))
	return fswatch.NewSession(watchOpt.dirs, handleFileEvent, opts...)
}

func startSession(session *fswatch.Session) {
	err := session.Start()
	if err != nil {
		logger.Error("failed to start watcher", zap.Error(err))
		logger.Warn("make sure you are not reaching your system's max number of open files")
	}
}

func stopSession(session *fswatch.Session) {
	if err := session.Stop(); err != nil {
        logger.Error("failed to stop watcher", zap.Error(err))
    }
    if err := session.Destroy(); err != nil {
        logger.Error("failed to destroy watcher", zap.Error(err))
    }
}

func registerEventHandler(watchOpt *watchOpt, triggerWatcher chan struct{}) func([]fswatch.Event) {
	return func(events []fswatch.Event) {
		for _, event := range events {
			if watchOpt.allowReload(event.Path){
				logger.Debug("filesystem change detected", zap.String("path", event.Path))
                triggerWatcher <- struct{}{}
                break
            }
		}
	}
}

func listenForFileEvents(trigger chan struct{}, stop chan struct{}) {
	timer := time.NewTimer(debounceDuration)
	timer.Stop()
	defer timer.Stop()
	for {
	    select {
	        case <-stop:
	            break
	        case <-trigger:
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
