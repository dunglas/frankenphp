//go:build !nowatcher

package watcher

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sgtdi/fswatcher"
)

const (
	// duration to wait before triggering a reload after a file change
	debounceDuration = 150 * time.Millisecond
	// times to retry watching if the watcher was closed prematurely
	maxFailureCount      = 5
	failureResetDuration = 5 * time.Second
)

var (
	ErrAlreadyStarted = errors.New("watcher is already running")

	failureMu       sync.Mutex
	watcherIsActive atomic.Bool

	// the currently active file watcher
	activeWatcher *globalWatcher
	// after stopping the watcher we will wait for eventual reloads to finish
	reloadWaitGroup sync.WaitGroup
	reloadMutex sync.Mutex
	// we are passing the context from the main package to the watcher
	globalCtx context.Context
	globalCancel context.CancelFunc
	// we are passing the globalLogger from the main package to the watcher
	globalLogger *slog.Logger
)

type PatternGroup struct {
	Patterns []string
	Callback func([]fswatcher.WatchEvent)
	fsw      fswatcher.Watcher
}

type globalWatcher struct {
	groups   []*PatternGroup
}

func InitWatcher(ct context.Context, slogger *slog.Logger, groups []*PatternGroup) error {
	if len(groups) == 0 {
		return nil
	}

	if watcherIsActive.Load() {
		return ErrAlreadyStarted
	}

	watcherIsActive.Store(true)
	globalCtx, globalCancel = context.WithCancel(ct)
	globalLogger = slogger

	activeWatcher = &globalWatcher{groups: groups}

	for _, pg := range groups {
		if len(pg.Patterns) > 0 {
			pg.startWatcher()
		}
	}

	return nil
}

func DrainWatcher() {
	if !watcherIsActive.Load() {
		return
	}

	watcherIsActive.Store(false)

	if globalLogger.Enabled(globalCtx, slog.LevelDebug) {
		globalLogger.LogAttrs(globalCtx, slog.LevelDebug, "stopping watcher")
	}

	globalCancel()
	reloadWaitGroup.Wait()
	activeWatcher = nil
}

func (pg *PatternGroup) startWatcher() {
	opts := []fswatcher.WatcherOpt{
		fswatcher.WithSeverity(fswatcher.SeverityDebug),
		fswatcher.WithCooldown(150*time.Millisecond),
	}
	for _, pattern := range pg.Patterns {
		dir, absPattern, err := extractRootFromPattern(pattern)
			if err != nil {
			globalLogger.Error("invalid watcher pattern", "error", err, "pattern", pattern)
			continue
		}
		regex, err := wildcardToRegex(absPattern)
		globalLogger.Warn("converted wildcard to regex", "pattern", absPattern, "regex", regex)
		if err != nil {
			globalLogger.Error("failed to convert wildcard to regex", "error", err)
			continue
		}

		opts = append(opts, fswatcher.WithPath(dir, fswatcher.WithPathIncRegex(regex)))
	}

	fsw, err := fswatcher.New(opts...)

	if err != nil {
		globalLogger.Error("failed to create watcher", "error", err)
		return
	}

	pg.fsw = fsw
    go pg.runWatcher()
	go pg.listenForFileEvents()
}

func (pg *PatternGroup) runWatcher() {
	err := pg.fsw.Watch(globalCtx)
		if err != nil {
			globalLogger.Error("failed to start watching", "error", err, "patternGroup", pg.Patterns)
		}
}

func (pg *PatternGroup) listenForFileEvents() {
	eventChan := pg.fsw.Events()

	for {
		debouncedEvents := make([]fswatcher.WatchEvent, 0)
		event := <-eventChan
		debouncedEvents = append(debouncedEvents, event)

		// drain the event channel to send events back as a group
	DrainLoop:
		for {
			select {
			case event := <-eventChan:
				debouncedEvents = append(debouncedEvents, event)
			default:
				break DrainLoop
			}
		}

		if globalLogger.Enabled(globalCtx, slog.LevelInfo) {
			globalLogger.LogAttrs(globalCtx, slog.LevelInfo, "filesystem changes detected", slog.Any("events", debouncedEvents))
		}

		pg.triggerCallbacks(debouncedEvents)

		if !pg.fsw.IsRunning() {
			break
		}
	}
}

func (pg *PatternGroup) triggerCallbacks(events []fswatcher.WatchEvent) {
	reloadMutex.Lock()
	reloadWaitGroup.Add(1)

	pg.Callback(events)

	reloadWaitGroup.Done()
	reloadMutex.Unlock()
}
