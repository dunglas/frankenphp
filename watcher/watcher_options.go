package watcher

import (
	"go.uber.org/zap"
	"path/filepath"
)

type WithWatchOption func(o *WatchOpt) error

type WatchOpt struct {
	dirs        []string
	isRecursive bool
	pattern     string
	trigger     chan struct{}
}

func WithWatcherDirs(dirs []string) WithWatchOption {
	return func(o *WatchOpt) error {
		for _, dir := range dirs {
			absDir, err := parseAbsPath(dir)
			if err != nil {
				return err
			}
			o.dirs = append(o.dirs, absDir)
		}
		return nil
	}
}

func WithWatcherRecursion(withRecursion bool) WithWatchOption {
	return func(o *WatchOpt) error {
		o.isRecursive = withRecursion
		return nil
	}
}

func WithWatcherPattern(pattern string) WithWatchOption {
	return func(o *WatchOpt) error {
		o.pattern = pattern
		return nil
	}
}

func parseAbsPath(path string) (string, error) {
	absDir, err := filepath.Abs(path)
	if err != nil {
		logger.Error("path could not be watched", zap.String("path", path), zap.Error(err))
		return "", err
	}
	return absDir, nil
}

// TODO: support directory patterns
func (watchOpt *WatchOpt) allowReload(fileName string, eventType int, pathType int) bool {
	if !isValidEventType(eventType) || !isValidPathType(pathType) {
		return false
	}
	if watchOpt.pattern == "" {
		return true
	}
	baseName := filepath.Base(fileName)
	patternMatches, err := filepath.Match(watchOpt.pattern, baseName)
	if err != nil {
		logger.Error("failed to match filename", zap.String("file", fileName), zap.Error(err))
		return false
	}
	if watchOpt.isRecursive {
		return patternMatches
	}
	fileNameDir := filepath.Dir(fileName)
	for _, dir := range watchOpt.dirs {
		if dir == fileNameDir {
			return patternMatches
		}
	}
	return false
}

// 0:rename,1:modify,2:create,3:destroy,4:owner,5:other,
func isValidEventType(eventType int) bool {
	return eventType <= 3
}

// 0:dir,1:file,2:hard_link,3:sym_link,4:watcher,5:other,
func isValidPathType(eventType int) bool {
	return eventType <= 2
}

