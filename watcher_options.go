package frankenphp

import (
	fswatch "github.com/dunglas/go-fswatch"
	"go.uber.org/zap"
	"path/filepath"
	"strings"
	"time"
)

type WatchOption func(o *watchOpt) error

type watchOpt struct {
	dirs            []string
	isRecursive     bool
	followSymlinks  bool
	isActive        bool
	latency         float64
	wildCardPattern string
	filters         []fswatch.Filter
	monitorType     fswatch.MonitorType
	eventTypes      []fswatch.EventType
}

func getDefaultWatchOpt() watchOpt {
	return watchOpt{
		isActive:    true,
		isRecursive: true,
		latency:     0.15,
		monitorType: fswatch.SystemDefaultMonitor,
		eventTypes:  parseEventTypes(),
	}
}

func WithWatcherShortForm(fileName string) WatchOption {
	return func(o *watchOpt) error {
		return parseShortForm(o, fileName)
	}
}

func WithWatcherDirs(dirs []string) WatchOption {
	return func(o *watchOpt) error {
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

func WithWatcherFilters(includeFiles string, excludeFiles string, caseSensitive bool, extendedRegex bool) WatchOption {
	return func(o *watchOpt) error {
		o.filters = parseFilters(includeFiles, excludeFiles, caseSensitive, extendedRegex)
		return nil
	}
}

func WithWatcherLatency(latency int) WatchOption {
	return func(o *watchOpt) error {
		o.latency = (float64)(latency) * time.Millisecond.Seconds()
		return nil
	}
}

func WithWatcherMonitorType(monitorType string) WatchOption {
	return func(o *watchOpt) error {
		o.monitorType = parseMonitorType(monitorType)
		return nil
	}
}

func WithWatcherRecursion(withRecursion bool) WatchOption {
	return func(o *watchOpt) error {
		o.isRecursive = withRecursion
		return nil
	}
}

func WithWatcherSymlinks(withSymlinks bool) WatchOption {
	return func(o *watchOpt) error {
		o.followSymlinks = withSymlinks
		return nil
	}
}

func WithWildcardPattern(pattern string) WatchOption {
	return func(o *watchOpt) error {
		o.wildCardPattern = pattern
		return nil
	}
}

// for the one line shortform in the caddy config, aka: 'watch /path/*pattern'
func parseShortForm(watchOpt *watchOpt, fileName string) error {
	watchOpt.isRecursive = true
	dirName := fileName
	splitDirName, baseName := filepath.Split(fileName)
	if fileName != "." && fileName != ".." && strings.ContainsAny(baseName, "*.") {
		dirName = splitDirName
		watchOpt.wildCardPattern = baseName
		watchOpt.isRecursive = false
	}

	if strings.Contains(fileName, "/**/") {
		dirName = strings.Split(fileName, "/**/")[0]
		watchOpt.wildCardPattern = strings.Split(fileName, "/**/")[1]
		watchOpt.isRecursive = true
	}

	absDir, err := parseAbsPath(dirName)
	if err != nil {
		return err
	}
	watchOpt.dirs = []string{absDir}
	return nil
}

func parseFilters(include string, exclude string, caseSensitive bool, extended bool) []fswatch.Filter {
	filters := []fswatch.Filter{}

	if include != "" && exclude == "" {
		exclude = "\\."
	}

	if include != "" {
		includeFilter := fswatch.Filter{
			Text:          include,
			FilterType:    fswatch.FilterInclude,
			CaseSensitive: caseSensitive,
			Extended:      extended,
		}
		filters = append(filters, includeFilter)
	}

	if exclude != "" {
		excludeFilter := fswatch.Filter{
			Text:          exclude,
			FilterType:    fswatch.FilterExclude,
			CaseSensitive: caseSensitive,
			Extended:      extended,
		}
		filters = append(filters, excludeFilter)
	}
	return filters
}

func parseMonitorType(monitorType string) fswatch.MonitorType {
	switch monitorType {
	case "fsevents":
		return fswatch.FseventsMonitor
	case "kqueue":
		return fswatch.KqueueMonitor
	case "inotify":
		return fswatch.InotifyMonitor
	case "windows":
		return fswatch.WindowsMonitor
	case "poll":
		return fswatch.PollMonitor
	case "fen":
		return fswatch.FenMonitor
	default:
		return fswatch.SystemDefaultMonitor
	}
}

func parseEventTypes() []fswatch.EventType {
	return []fswatch.EventType{
		fswatch.Created,
		fswatch.Updated,
		fswatch.Renamed,
		fswatch.Removed,
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

func (watchOpt *watchOpt) allowReload(fileName string) bool {
	if !watchOpt.isActive {
		return false
	}
	if watchOpt.wildCardPattern == "" {
		return true
	}
	baseName := filepath.Base(fileName)
	patternMatches, err := filepath.Match(watchOpt.wildCardPattern, baseName)
	if err != nil {
		logger.Error("failed to match filename", zap.String("file", fileName), zap.Error(err))
		return false
	}
	return patternMatches
}
