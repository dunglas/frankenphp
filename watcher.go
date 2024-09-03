package frankenphp

import (
	fswatch "github.com/dunglas/go-fswatch"
	"go.uber.org/zap"
	"time"
	"sync/atomic"
)

type watchEvent struct {
	events []fswatch.Event
	watchOpt watchOpt
}

// sometimes multiple events fire at once so we'll wait a few ms before reloading
const debounceDuration = 100
// latency of the watcher in seconds
const watcherLatency = 0.1

var (
	watchSessions         []*fswatch.Session
	blockReloading        atomic.Bool
	fileEventChannel      chan watchEvent
	watcherDone           chan interface{}
)

func initWatcher(watchOpts []watchOpt, workerOpts []workerOpt) error {
	if(len(watchOpts) == 0 || len(workerOpts) == 0) {
		return nil
	}
	blockReloading.Store(true)

	watchSessions := make([]*fswatch.Session, len(watchOpts))
	for i, watchOpt := range watchOpts {
		session, err := createSession(watchOpt)
		if(err != nil) {
			return err
		}
		watchSessions[i] = session
		go session.Start()
	}
	watcherDone = make(chan interface{} )
	fileEventChannel = make(chan watchEvent)
	for _, session := range watchSessions {
    	go session.Start()
    }

	go listenForFileChanges(watchOpts, workerOpts)
	blockReloading.Store(false)
	return nil;
}

func createSession(watchOpt watchOpt) (*fswatch.Session, error) {
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
		fswatch.WithLatency(0.01),
	}
	return fswatch.NewSession([]string{watchOpt.dirName}, registerFileEvent(watchOpt), opts...)
}

func stopWatcher() {
	if watcherDone == nil {
		return
	}
	logger.Info("stopping watcher")
	blockReloading.Store(true)
	close(watcherDone)
	watcherDone = nil
	for _, session := range watchSessions {
		if err := session.Destroy(); err != nil {
			panic(err)
		}
	}
}

func listenForFileChanges(watchOpts []watchOpt, workerOpts []workerOpt) {
	for {
		select {
		case watchEvent := <-fileEventChannel:
			for _, event := range watchEvent.events {
				handleFileEvent(event, watchEvent.watchOpt, workerOpts)
			}
		case <-watcherDone:
			return
		}
	}
}


func registerFileEvent(watchOpt watchOpt) func([]fswatch.Event) {
	return func(events []fswatch.Event) {
		fileEventChannel <- watchEvent{events,watchOpt}
	}
}

func handleFileEvent(event fswatch.Event, watchOpt watchOpt, workerOpts []workerOpt) {
	if blockReloading.Load() || !fileMatchesPattern(event.Path, watchOpt) {
		return
	}
	blockReloading.Store(true)
	logger.Info("filesystem change detected", zap.String("path", event.Path))
	go reloadWorkers(workerOpts)
}

	
func reloadWorkers(workerOpts []workerOpt) {
	<-time.After(time.Millisecond * time.Duration(debounceDuration))

	logger.Info("restarting workers due to file changes...")
	stopWorkers()
	if err := initWorkers(workerOpts); err != nil {
		logger.Error("failed to restart workers when watching files")
		panic(err)
	}

	logger.Info("workers restarted successfully")
	blockReloading.Store(false)
}


