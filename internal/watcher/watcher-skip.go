//go:build nowatcher

package watcher

import "log/slog"

func InitWatcher(filePatterns []string, callback func(), logger *slog.Logger) error {
	logger.Error("watcher support is not enabled")

	return nil
}

func DrainWatcher() {
}
