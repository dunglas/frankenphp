package watcher

import (
	"go.uber.org/zap"
	"path/filepath"
	"strings"
)

type watchOpt struct {
	dir         string
	isRecursive bool
	pattern     string
	trigger     chan struct{}
}

func parseFilePatterns(filePatterns []string) ([]*watchOpt, error) {
	watchOpts := make([]*watchOpt, 0, len(filePatterns))
	for _, filePattern := range filePatterns {
		watchOpt, err := parseFilePattern(filePattern)
		if err != nil {
			return nil, err
		}
		watchOpts = append(watchOpts, watchOpt)
	}
	return watchOpts, nil
}

// TODO: better path validation?
// for the one line short-form in the caddy config, aka: 'watch /path/*pattern'
func parseFilePattern(filePattern string) (*watchOpt, error) {
	absPattern, err := filepath.Abs(filePattern)
	if err != nil {
		return nil, err
	}

	var w watchOpt
	w.isRecursive = true
	dirName := absPattern
	splitDirName, baseName := filepath.Split(absPattern)
	if strings.Contains(absPattern, "**") {
		split := strings.Split(absPattern, "**")
        dirName = split[0]
        w.pattern = strings.TrimLeft(split[1], "/")
        w.isRecursive = true
    } else if strings.ContainsAny(baseName, "*.[?\\") {
		dirName = splitDirName
		w.pattern = baseName
		w.isRecursive = false
	}

	w.dir = dirName
	if dirName != "/" {
		w.dir = strings.TrimRight(dirName, "/")
	}

	return &w, nil
}

// TODO: support directory patterns
func (watchOpt *watchOpt) allowReload(fileName string, eventType int, pathType int) bool {
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
