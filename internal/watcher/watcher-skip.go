//go:build nowatcher

package watcher

import "go.uber.org/zap"

func InitWatcher(filePatterns []string, callback func(), zapLogger *zap.Logger) error {
	zapLogger.Error("watcher support is not enabled")

	return nil
}

func DrainWatcher() {
}
