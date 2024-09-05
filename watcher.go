package frankenphp

import (
	fswatch "github.com/dunglas/go-fswatch"
	"go.uber.org/zap"
	"sync/atomic"
	"sync"
)

// latency of the watcher in seconds
const watcherLatency = 0.15

var (
	watchSessions      []*fswatch.Session
	blockReloading     atomic.Bool
	reloadWaitGroup    sync.WaitGroup
)

func initWatcher(watchOpts []watchOpt, workerOpts []workerOpt) error {
	if(len(watchOpts) == 0 || len(workerOpts) == 0) {
		return nil
	}

	watchSessions := make([]*fswatch.Session, len(watchOpts))
	for i, watchOpt := range watchOpts {
		session, err := createSession(watchOpt, workerOpts)
		if(err != nil) {
			return err
		}
		watchSessions[i] = session
	}

	for _, session := range watchSessions {
		go session.Start()
	}

	blockReloading.Store(false)
	reloadWaitGroup = sync.WaitGroup{}
	return nil;
}

func createSession(watchOpt watchOpt, workerOpts []workerOpt) (*fswatch.Session, error) {
	eventTypeFilters := []fswatch.EventType{
		fswatch.Created,
		fswatch.Updated,
		fswatch.Renamed,
		fswatch.Removed,
	}
	// Todo: allow more fine grained control over the options
	opts := []fswatch.Option{
		fswatch.WithRecursive(watchOpt.isRecursive),
		fswatch.WithFollowSymlinks(false),
		fswatch.WithEventTypeFilters(eventTypeFilters),
		fswatch.WithLatency(watcherLatency),
	}
	return fswatch.NewSession([]string{watchOpt.dirName}, registerFileEvent(watchOpt, workerOpts), opts...)
}

func stopWatcher() {
	if(len(watchSessions) == 0) {
		return
	}
	logger.Info("stopping watcher")
	blockReloading.Store(true)
	for _, session := range watchSessions {
		if err := session.Stop(); err != nil {
            logger.Error("failed to stop watcher")
        }
		if err := session.Destroy(); err != nil {
			logger.Error("failed to destroy watcher")
		}
	}
	reloadWaitGroup.Wait()
}

func registerFileEvent(watchOpt watchOpt, workerOpts []workerOpt) func([]fswatch.Event) {
	return func(events []fswatch.Event) {
		for _, event := range events {
			if (handleFileEvent(event, watchOpt, workerOpts)){
				return
			}
		}
	}
}

func handleFileEvent(event fswatch.Event, watchOpt watchOpt, workerOpts []workerOpt) bool {
	if !fileMatchesPattern(event.Path, watchOpt) || !blockReloading.CompareAndSwap(false, true) {
		return false
	}
	reloadWaitGroup.Wait()
	reloadWaitGroup.Add(1)
	logger.Info("filesystem change detected", zap.String("path", event.Path))
	go reloadWorkers(workerOpts)
	return true
}

	
func reloadWorkers(workerOpts []workerOpt) {
	logger.Info("restarting workers due to file changes...")
	stopWorkers()
	blockReloading.Store(false)
	if err := initWorkers(workerOpts); err != nil {
		logger.Error("failed to restart workers when watching files")
		panic(err)
	}

	logger.Info("workers restarted successfully")
	reloadWaitGroup.Done()
}


