package frankenphp

import (
	fswatch "github.com/dunglas/go-fswatch"
	"go.uber.org/zap"
	"sync"
	"sync/atomic"
)

type watcher struct {
	sessions   []*fswatch.Session
	watchOpts  []*watchOpt
	workerOpts []workerOpt
}

var (
	// the currently active file watcher
	activeWatcher *watcher
	// reloading is blocked if a reload is queued
	blockReloading atomic.Bool
	// when stopping the watcher we need to wait for reloading to finish
	reloadWaitGroup sync.WaitGroup
)

func initWatcher(watchOpts []watchOpt, workerOpts []workerOpt) error {
	if len(watchOpts) == 0 || len(workerOpts) == 0 {
		return nil
	}
	activeWatcher = &watcher{workerOpts: workerOpts}
	err := activeWatcher.startWatching(watchOpts)
	if err != nil {
		return err
	}
	reloadWaitGroup = sync.WaitGroup{}
	blockReloading.Store(false)

	return nil
}

func drainWatcher() {
	if(activeWatcher == nil) {
        return
    }
	logger.Info("stopping watcher...")
	blockReloading.Store(true)
	activeWatcher.stopWatching()
	reloadWaitGroup.Wait()
	activeWatcher = nil
}

func (w *watcher) startWatching(watchOpts []watchOpt) error {
	w.sessions = make([]*fswatch.Session, len(watchOpts))
	w.watchOpts = make([]*watchOpt, len(watchOpts))
	for i, watchOpt := range watchOpts {
        session, err := createSession(&watchOpt)
        if err != nil {
            logger.Error("unable to watch dirs", zap.Strings("dirs", watchOpt.dirs))
            return err
        }
		w.watchOpts[i] = &watchOpt
        w.sessions[i] = session
        go func() {
            err := session.Start()
            if err != nil {
            	logger.Error("failed to start watcher", zap.Error(err))
            	logger.Warn("make sure you are not reaching your system's max number of open files")
			}
        }()
    }
	return nil
}

func (w *watcher) stopWatching() {
	for i, session := range w.sessions {
        w.watchOpts[i].isActive = false
        if err := session.Stop(); err != nil {
            logger.Error("failed to stop watcher", zap.Error(err))
        }
        if err := session.Destroy(); err != nil {
            logger.Error("failed to destroy watcher", zap.Error(err))
        }
    }
}

func createSession(watchOpt *watchOpt) (*fswatch.Session, error) {
	opts := []fswatch.Option{
		fswatch.WithRecursive(watchOpt.isRecursive),
		fswatch.WithFollowSymlinks(watchOpt.followSymlinks),
		fswatch.WithEventTypeFilters(watchOpt.eventTypes),
		fswatch.WithLatency(watchOpt.latency),
		fswatch.WithMonitorType((fswatch.MonitorType)(watchOpt.monitorType)),
		fswatch.WithFilters(watchOpt.filters),
	}
	handleFileEvent := registerFileEvent(watchOpt)
	logger.Debug("starting watcher session", zap.Strings("dirs", watchOpt.dirs))
	return fswatch.NewSession(watchOpt.dirs, handleFileEvent, opts...)
}

func registerFileEvent(watchOpt *watchOpt) func([]fswatch.Event) {
	return func(events []fswatch.Event) {
		for _, event := range events {
			if handleFileEvent(event, watchOpt) {
				break
			}
		}
	}
}

func handleFileEvent(event fswatch.Event, watchOpt *watchOpt) bool {
	if !watchOpt.allowReload(event.Path) || !blockReloading.CompareAndSwap(false, true) {
		return false
	}
	logger.Info("filesystem change detected, restarting workers...", zap.String("path", event.Path))
	go triggerWorkerReload()

	return true
}

func triggerWorkerReload() {
	reloadWaitGroup.Add(1)
	restartWorkers(activeWatcher.workerOpts)
	reloadWaitGroup.Done()
	blockReloading.Store(false)
}
