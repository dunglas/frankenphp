package watcher

import (
	"go.uber.org/zap"
	"path/filepath"
)

type WithWatchOption func(o *WatchOpt) error

type WatchOpt struct {
	dir         string
	isRecursive bool
	pattern     string
	trigger     chan struct{}
}

func WithWatcherDir(dir string) WithWatchOption {
	return func(o *WatchOpt) error {
		absDir, err := filepath.Abs(dir)
		if err != nil {
			logger.Error("dir for watching is invalid", zap.String("dir", dir))
			return err
		}
		o.dir = absDir
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

// TODO: support directory patterns
func (watchOpt *WatchOpt) allowReload(fileName string, eventType int, pathType int) bool {
	if !isValidEventType(eventType) || !isValidPathType(pathType) {
		return false
	}
	if watchOpt.isRecursive {
		return isValidRecursivePattern(fileName, watchOpt.pattern)
	}
	return isValidNonRecursivePattern(fileName, watchOpt.pattern, watchOpt.dir)
}

// 0:rename,1:modify,2:create,3:destroy,4:owner,5:other,
func isValidEventType(eventType int) bool {
	return eventType <= 3
}

// 0:dir,1:file,2:hard_link,3:sym_link,4:watcher,5:other,
func isValidPathType(eventType int) bool {
	return eventType <= 2
}

func isValidRecursivePattern(fileName string, pattern string) bool {
	if pattern == "" {
		return true
	}
	baseName := filepath.Base(fileName)
	patternMatches, err := filepath.Match(pattern, baseName)
	if err != nil {
		logger.Error("failed to match filename", zap.String("file", fileName), zap.Error(err))
		return false
	}

	return patternMatches
}

func isValidNonRecursivePattern(fileName string, pattern string, dir string) bool {
	fileNameDir := filepath.Dir(fileName)
	if dir == fileNameDir {
		return isValidRecursivePattern(fileName, pattern)
	}

	return false
}
