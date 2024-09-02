package frankenphp

import (
	"github.com/fsnotify/fsnotify"
	"go.uber.org/zap"
	"path/filepath"
	"strings"
	"os"
	"time"
)

// sometimes multiple fs events fire at once so we'll wait a few ms before reloading
const debounceDuration = 300
var watcher *fsnotify.Watcher
var isReloadingWorkers bool = false

func initWatcher(watchOpts []watchOpt, workerOpts []workerOpt) error {
	if(len(watchOpts) == 0) {
		return nil
	}
	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	watcher = fsWatcher;

	go listenForFileChanges(watchOpts, workerOpts)

	if err := addWatchedDirectories(watchOpts); err != nil {
		logger.Error("failed to watch directories")
		return err
	}

	return nil;
}

func stopWatcher() {
	if(watcher != nil) {
		watcher.Close()
	}
}

func listenForFileChanges(watchOpts []watchOpt, workerOpts []workerOpt) {
	for {
        select {
        case event, ok := <-watcher.Events:
            if !ok {
                logger.Error("unexpected watcher event")
                return
            }
            watchCreatedDirectories(event, watchOpts)
            if isReloadingWorkers || !fileMatchesPattern(event.Name, watchOpts) {
                continue
            }
            isReloadingWorkers = true
            logger.Info("filesystem change detected", zap.String("event", event.Name))
            go reloadWorkers(workerOpts)

        case err, ok := <-watcher.Errors:
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
			watcher.Add(watchOpt.dirName)
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
		watcher.Add(dir)
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
            watcher.Add(event.Name)
		}
	}

}

func watchFile(path string, fi os.FileInfo, err error) error {
	// ignore paths that start with a dot (like .git)
	if fi.Mode().IsDir() && !strings.HasPrefix(filepath.Base(fi.Name()), ".") {
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
	isReloadingWorkers = false
}

