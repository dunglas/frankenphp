//go:build !nowatcher

package watcher

import (
	"context"
	"errors"
	"log/slog"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"path/filepath"
	"time"

	"github.com/sgtdi/fswatcher"
)

var (
	ErrAlreadyStarted = errors.New("watcher is already running")

	// the currently active file watcher
	activeWatcher   *globalWatcher
	watcherIsActive atomic.Bool

	// we are passing the globalLogger from the main package to the watcher
	globalLogger  *slog.Logger
	globalContext context.Context
)

type compiledPattern struct {
	root  string
	regex *regexp.Regexp // nil: match any path under root
}

type PatternGroup struct {
	Patterns []string
	Callback func([]fswatcher.WatchEvent)

	matchers []compiledPattern
}

type globalWatcher struct {
	groups    []*PatternGroup
	fsw       fswatcher.Watcher
	done      chan any
	mu        sync.Mutex
	waitGroup sync.WaitGroup
}

func InitWatcher(ct context.Context, slogger *slog.Logger, groups []*PatternGroup) error {
	if len(groups) == 0 {
		return nil
	}

	if !watcherIsActive.CompareAndSwap(false, true) {
		return ErrAlreadyStarted
	}

	activeWatcher = &globalWatcher{groups: groups}
	globalContext = ct
	globalLogger = slogger

	if err := activeWatcher.start(); err != nil {
		activeWatcher = nil
		watcherIsActive.Store(false)
		return err
	}

	return nil
}

func DrainWatcher() {
	if !watcherIsActive.Load() {
		return
	}

	if globalLogger.Enabled(globalContext, slog.LevelDebug) {
		globalLogger.LogAttrs(globalContext, slog.LevelDebug, "stopping watcher")
	}

	if activeWatcher != nil {
		if activeWatcher.fsw != nil {
			activeWatcher.fsw.Close()
		}
		if activeWatcher.done != nil {
			close(activeWatcher.done)
		}
	}

	watcherIsActive.Store(false)
	activeWatcher.mu.Lock()
	activeWatcher.mu.Unlock()
	globalLogger.Info("waiting for watcher to finish")
	activeWatcher.waitGroup.Wait()
	globalLogger.Info("watcher finished")

	activeWatcher = nil
}

func (pg *PatternGroup) compileMatchers() {
	for _, pattern := range pg.Patterns {
		root, regexStr, err := extractRootAndRegex(pattern)
		if err != nil {
			globalLogger.Error("invalid watcher pattern, unable to watch", "error", err, "pattern", pattern)
			continue
		}

		cp := compiledPattern{root: root}
		if regexStr != "" {
			re, err := regexp.Compile(regexStr)
			if err != nil {
				globalLogger.Error("invalid watcher pattern regex", "error", err, "pattern", pattern)
				continue
			}
			cp.regex = re
		}
		pg.matchers = append(pg.matchers, cp)
	}
}

func (pg *PatternGroup) matchesPath(path string) bool {
	for _, m := range pg.matchers {
		if m.regex == nil {
			if strings.HasPrefix(path, m.root) {
				return true
			}
			continue
		}
		path = filepath.ToSlash(path)
		if m.regex.MatchString(path) {
			return true
		}
	}

	return false
}

func (gw *globalWatcher) start() error {
	opts := []fswatcher.WatcherOpt{
		fswatcher.WithSeverity(fswatcher.SeverityDebug),
		fswatcher.WithCooldown(150 * time.Millisecond),
	}

	watchedDirs := make(map[string]struct{})
	for _, pg := range gw.groups {
		if len(pg.Patterns) == 0 {
			continue
		}
		pg.compileMatchers()
		for _, m := range pg.matchers {
			if _, ok := watchedDirs[m.root]; ok {
				continue
			}
			watchedDirs[m.root] = struct{}{}
			opts = append(opts, fswatcher.WithPath(m.root))
		}
	}

	if len(watchedDirs) == 0 {
		return errors.New("no valid watch patterns")
	}

	fsw, err := fswatcher.New(opts...)
	if err != nil {
		globalLogger.Error("failed to create watcher", "error", err)
		return err
	}

	gw.fsw = fsw
	gw.done = make(chan any)
	gw.waitGroup.Add(1)
	go gw.runWatcher()
	go gw.listenForFileEvents()

	return nil
}

func (gw *globalWatcher) runWatcher() {
	err := gw.fsw.Watch(globalContext)
	if err != nil {
		globalLogger.Error("failed to start watching", "error", err)
	}
}

func (gw *globalWatcher) listenForFileEvents() {
	defer gw.waitGroup.Done()

	eventChan := gw.fsw.Events()

	for {
		debouncedEvents := make([]fswatcher.WatchEvent, 0)
		select {
		case event := <-eventChan:
			debouncedEvents = append(debouncedEvents, event)
		case <-gw.done:
			return
		}

	DrainLoop:
		for {
			select {
			case event := <-eventChan:
				debouncedEvents = append(debouncedEvents, event)
			default:
				break DrainLoop
			}
		}

		changesDetected := false
		for _, pg := range gw.groups {
			if len(pg.matchers) == 0 {
				continue
			}
			matched := make([]fswatcher.WatchEvent, 0)
			for _, event := range debouncedEvents {
				if pg.matchesPath(event.Path) {
					matched = append(matched, event)
				}
			}
			if len(matched) > 0 {
				if !changesDetected && globalLogger.Enabled(globalContext, slog.LevelInfo) {
					globalLogger.LogAttrs(globalContext, slog.LevelInfo, "filesystem changes detected", slog.Any("events", debouncedEvents))
				}
				changesDetected = true
				pg.triggerCallbacks(matched)
			}
		}

		if !gw.fsw.IsRunning() || !watcherIsActive.Load() {
			break
		}
	}
}

func (pg *PatternGroup) triggerCallbacks(events []fswatcher.WatchEvent) {
	activeWatcher.mu.Lock()
	defer activeWatcher.mu.Unlock()

	if watcherIsActive.Load() {
		pg.Callback(events)
	}
}
