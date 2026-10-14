//go:build !nowatcher

package frankenphp

import (
	"github.com/dunglas/frankenphp/internal/watcher"
	"github.com/sgtdi/fswatcher"
)

type hotReloadOpt struct {
	hotReload []*watcher.PatternGroup
}

// validateWatchers has nothing to refuse where the watcher is compiled in:
// the patterns are only known to be good once the watcher took them
func validateWatchers(*opt) error {
	return nil
}

func (o *hotReloadOpt) hasHotReload() bool {
	return len(o.hotReload) != 0
}

func initWatchers(o *opt) error {
	allPatterns := make([]*watcher.PatternGroup, 0, len(o.hotReload))
	workerPattern := &watcher.PatternGroup{
		Callback: func(_ []fswatcher.WatchEvent) {
			RestartWorkers()
		},
	}

	for _, o := range o.workers {
		if len(o.watch) == 0 {
			continue
		}

		watcherIsEnabled = true
		workerPattern.Patterns = append(workerPattern.Patterns, o.watch...)
	}

	if watcherIsEnabled {
		allPatterns = append(allPatterns, workerPattern)
	}

	return watcher.InitWatcher(globalCtx, globalLogger, append(allPatterns, o.hotReload...))
}

func drainWatchers() {
	watcher.DrainWatcher()
}
