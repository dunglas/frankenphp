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

	// the currently active file watcher
	activeWatcher   *globalWatcher
	watcherIsActive atomic.Bool

	// we are passing the globalLogger from the main package to the watcher
	globalLogger  *slog.Logger
	globalContext context.Context
)

type PatternGroup struct {
	Patterns []string
	Callback func([]fswatcher.WatchEvent)
	fsw      fswatcher.Watcher
	done     chan any
}

type globalWatcher struct {
	groups    []*PatternGroup
	mu        sync.Mutex
	waitGroup sync.WaitGroup
}

func InitWatcher(ct context.Context, slogger *slog.Logger, groups []*PatternGroup) error {
	if len(groups) == 0 {
		return nil
	}

	if watcherIsActive.Load() {
		return ErrAlreadyStarted
	}

	watcherIsActive.Store(true)
	activeWatcher = &globalWatcher{groups: groups, ctx: context.Background()}
	globalLogger = slogger

	for _, pg := range groups {
		if len(pg.Patterns) > 0 {
			pg.done = make(chan any)
			pg.startWatching()
		}
	}

	return nil
}

func DrainWatcher() {
	if !watcherIsActive.Load() {
		return
	}

	watcherIsActive.Store(false)

	if globalLogger.Enabled(globalContext, slog.LevelDebug) {
		globalLogger.LogAttrs(globalContext, slog.LevelDebug, "stopping watcher")
	}

	for _, pg := range activeWatcher.groups {
		pg.fsw.Close()
		close(pg.done)
	}
	activeWatcher.mu.Lock()
	activeWatcher.mu.Unlock()
	globalLogger.Info("waiting for watcher to finish")
	activeWatcher.waitGroup.Wait()
	globalLogger.Info("watcher finished")

	activeWatcher = nil
}

func (pg *PatternGroup) startWatching() {
	opts := []fswatcher.WatcherOpt{
		fswatcher.WithSeverity(fswatcher.SeverityDebug),
		fswatcher.WithCooldown(150 * time.Millisecond),
	}
	for _, pattern := range pg.Patterns {
		dir, regex, err := extractRootAndRegex(pattern)
		if err != nil {
			globalLogger.Error("invalid watcher pattern, unable to watch directory", "error", err, "pattern", pattern)
			continue
		}

		// pattern is a concrete directory, just watch it without any regex filtering
		if regex == "" {
			opts = append(opts, fswatcher.WithPath(dir))
		} else {
			opts = append(opts, fswatcher.WithPath(dir, fswatcher.WithPathIncRegex(regex)))
		}
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
	err := pg.fsw.Watch(globalContext)
	if err != nil {
		globalLogger.Error("failed to start watching", "error", err, "patternGroup", pg.Patterns)
	}
}

func (pg *PatternGroup) listenForFileEvents() {
	activeWatcher.waitGroup.Add(1)
	defer activeWatcher.waitGroup.Done()

	eventChan := pg.fsw.Events()

	for {
		debouncedEvents := make([]fswatcher.WatchEvent, 0)
		select {
		case event := <-eventChan:
			debouncedEvents = append(debouncedEvents, event)
		case <-pg.done:
			return
		}

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

		if globalLogger.Enabled(globalContext, slog.LevelInfo) {
			globalLogger.LogAttrs(globalContext, slog.LevelInfo, "filesystem changes detected", slog.Any("events", debouncedEvents))
		}

		pg.triggerCallbacks(debouncedEvents)

		if !pg.fsw.IsRunning() || !watcherIsActive.Load() {
			break
		}
	}
}

func (pg *PatternGroup) triggerCallbacks(events []fswatcher.WatchEvent) {
	activeWatcher.mu.Lock()

	if watcherIsActive.Load() {
		pg.Callback(events)
	}

	activeWatcher.mu.Unlock()
}
