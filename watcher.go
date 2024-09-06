package frankenphp

import (
	fswatch "github.com/dunglas/go-fswatch"
	"go.uber.org/zap"
	"sync"
	"sync/atomic"
)

var (
	// TODO: combine session and watchOpt into a struct
	watchSessions       []*fswatch.Session
	// we block reloading until workers have stopped
	blockReloading      atomic.Bool
	// when stopping the watcher we need to wait for reloading to finish
	reloadWaitGroup     sync.WaitGroup
	// active watch options that need to be disabled on shutdown
	activeWatchOpts     []*watchOpt
)

func initWatcher(watchOpts []watchOpt, workerOpts []workerOpt) error {
	if len(watchOpts) == 0 || len(workerOpts) == 0 {
		return nil
	}

	watchSessions := make([]*fswatch.Session, len(watchOpts))
	activeWatchOpts = make([]*watchOpt, len(watchOpts))
	for i, watchOpt := range watchOpts {
		session, err := createSession(&watchOpt, workerOpts)
		if err != nil {
			logger.Error("unable to start watcher", zap.Strings("dirs", watchOpt.dirs))
			return err
		}
		watchSessions[i] = session
		activeWatchOpts[i] = &watchOpt
		go session.Start()
	}

	reloadWaitGroup = sync.WaitGroup{}
	blockReloading.Store(false)
	return nil
}

func createSession(watchOpt *watchOpt, workerOpts []workerOpt) (*fswatch.Session, error) {
	opts := []fswatch.Option{
		fswatch.WithRecursive(watchOpt.isRecursive),
		fswatch.WithFollowSymlinks(watchOpt.followSymlinks),
		fswatch.WithEventTypeFilters(watchOpt.eventTypes),
		fswatch.WithLatency(watchOpt.latency),
		fswatch.WithMonitorType((fswatch.MonitorType)(watchOpt.monitorType)),
		fswatch.WithFilters(watchOpt.filters),
	}
	handleFileEvent := registerFileEvent(watchOpt, workerOpts)
	return fswatch.NewSession(watchOpt.dirs, handleFileEvent, opts...)
}

func drainWatcher() {
	stopWatcher()
	reloadWaitGroup.Wait()
}

func stopWatcher() {
	logger.Info("stopping watcher...")
	blockReloading.Store(true)
	for _, session := range watchSessions {
		if err := session.Stop(); err != nil {
			logger.Error("failed to stop watcher", zap.Error(err))
		}
		if err := session.Destroy(); err != nil {
			logger.Error("failed to destroy watcher", zap.Error(err))
		}
	}
	// we also need to deactivate the watchOpts to avoid a race condition in tests
	for _, watchOpt := range activeWatchOpts {
		watchOpt.isActive = false
	}
}

func registerFileEvent(watchOpt *watchOpt, workerOpts []workerOpt) func([]fswatch.Event) {
	return func(events []fswatch.Event) {
		for _, event := range events {
			if handleFileEvent(event, watchOpt, workerOpts) {
				break
			}
		}
	}
}

func handleFileEvent(event fswatch.Event, watchOpt *watchOpt, workerOpts []workerOpt) bool {
	if !watchOpt.allowReload(event.Path) || !blockReloading.CompareAndSwap(false, true) {
		return false
	}
	logger.Info("filesystem change detected, restarting workers...", zap.String("path", event.Path))
	go triggerWorkerReload(workerOpts)

	return true
}

func triggerWorkerReload(workerOpts []workerOpt) {
	reloadWaitGroup.Add(1)
	restartWorkers(workerOpts)
	reloadWaitGroup.Done()
	blockReloading.Store(false)
}
