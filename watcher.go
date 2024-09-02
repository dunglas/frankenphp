package frankenphp

import (
	"github.com/fsnotify/fsnotify"
	"go.uber.org/zap"
	"path/filepath"
	"strings"
	"os"
	"time"
	"sync/atomic"
	"sync"
)

// sometimes multiple fs events fire at once so we'll wait a few ms before reloading
const debounceDuration = 300
var watcher *fsnotify.Watcher
var watcherMu sync.RWMutex
var isReloadingWorkers atomic.Bool

func initWatcher(watchOpts []watchOpt, workerOpts []workerOpt) error {
	if(len(watchOpts) == 0) {
		return nil
	}
	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	watcherMu.Lock()
	watcher = fsWatcher;
	watcherMu.Unlock()

	if err := addWatchedDirectories(watchOpts); err != nil {
		logger.Error("failed to watch directories")
		return err
	}

	go listenForFileChanges(watchOpts, workerOpts)

	return nil;
}

func stopWatcher() {
	if(watcher != nil) {
		watcherMu.RLock()
		watcher.Close()
		watcherMu.RUnlock()
	}
}

func listenForFileChanges(watchOpts []watchOpt, workerOpts []workerOpt) {
	watcherMu.RLock()
	events := watcher.Events
	errors := watcher.Errors
	watcherMu.RUnlock()
	for {
		select {
		case event, ok := <-events:
			if !ok {
				logger.Error("unexpected watcher event")
				return
			}
			watchCreatedDirectories(event, watchOpts)
			if isReloadingWorkers.Load() || !fileMatchesPattern(event.Name, watchOpts) {
				continue
			}
			isReloadingWorkers.Store(true)
			logger.Info("filesystem change detected", zap.String("event", event.Name))
			go reloadWorkers(workerOpts)

		case err, ok := <-errors:
			if !ok {
				return
			}
			logger.Error("watcher: error:", zap.Error(err))
		}
	}
}


func addWatchedDirectories(watchOpts []watchOpt) error {
	for _, watchOpt := range watchOpts {
		logger.Debug("watching for changes", zap.String("dir", watchOpt.dirName), zap.String("pattern", watchOpt.pattern), zap.Bool("recursive", watchOpt.isRecursive))
		if(watchOpt.isRecursive == false) {
			watcherMu.RLock()
			watcher.Add(watchOpt.dirName)
			watcherMu.RUnlock()
			continue
		}
		if err := watchRecursively(watchOpt.dirName); err != nil {
			return err
		}
	}

	return nil
}

func watchRecursively(dir string) error {
	fileInfo, err := os.Stat(dir)
	if err != nil {
		return err
	}
	if !fileInfo.IsDir() {
		watcherMu.RLock()
		watcher.Add(dir)
		watcherMu.RUnlock()
		return nil
	}
	if err := filepath.Walk(dir, watchFile); err != nil {
		return err;
	}

	return nil
}

func watchCreatedDirectories(event fsnotify.Event, watchOpts []watchOpt) {
	if !event.Has(fsnotify.Create) {
		return
	}
	fileInfo, err := os.Stat(event.Name)
	if err != nil {
		logger.Error("unable to stat file", zap.Error(err))
		return
	}
	if fileInfo.IsDir() != true {
		return
	}
	for _, watchOpt := range watchOpts {
		if(watchOpt.isRecursive && strings.HasPrefix(event.Name, watchOpt.dirName)) {
			logger.Debug("watching new dir", zap.String("dir", event.Name))
			watcherMu.RLock()
			watcher.Add(event.Name)
			watcherMu.RLock()
		}
	}

}

func watchFile(path string, fi os.FileInfo, err error) error {
	// ignore paths that start with a dot (like .git)
	if fi.Mode().IsDir() && !strings.HasPrefix(filepath.Base(fi.Name()), ".") {
		watcherMu.RLock()
		defer watcherMu.RUnlock()
		return watcher.Add(path)
	}
	return nil
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
	isReloadingWorkers.Store(false)
}


