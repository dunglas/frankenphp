package caddy

import (
	"github.com/dunglas/frankenphp"
	"github.com/dunglas/frankenphp/watcher"
	"path/filepath"
	"strings"
)

type watchConfig struct {
	// Directory that should be watched for changes
	Dir string `json:"dir,omitempty"`
	// Whether to watch the directory recursively
	IsRecursive bool `json:"recursive,omitempty"`
	// The shell filename pattern to match against
	Pattern string `json:"pattern,omitempty"`
}

func applyWatchConfig(opts []frankenphp.Option, watchConfig watchConfig) []frankenphp.Option {
	return append(opts, frankenphp.WithFileWatcher(
		watcher.WithWatcherDir(watchConfig.Dir),
		watcher.WithWatcherRecursion(watchConfig.IsRecursive),
		watcher.WithWatcherPattern(watchConfig.Pattern),
	))
}

func parseWatchConfigs(filePatterns []string) []watchConfig {
	watchConfigs := []watchConfig{}
	for _, filePattern := range filePatterns {
		watchConfigs = append(watchConfigs, parseWatchConfig(filePattern))
	}
	return watchConfigs
}

// TODO: better path validation?
// for the one line short-form in the caddy config, aka: 'watch /path/*pattern'
func parseWatchConfig(filePattern string) watchConfig {
	watchConfig := watchConfig{IsRecursive: true}
	dirName := filePattern
	splitDirName, baseName := filepath.Split(filePattern)
	if filePattern != "." && filePattern != ".." && strings.ContainsAny(baseName, "*.[?\\") {
		dirName = splitDirName
		watchConfig.Pattern = baseName
		watchConfig.IsRecursive = false
	}
	if strings.Contains(filePattern, "**/") {
		dirName = strings.Split(filePattern, "**/")[0]
		watchConfig.Pattern = strings.Split(filePattern, "**/")[1]
		watchConfig.IsRecursive = true
	}
	watchConfig.Dir = dirName
	if dirName != "/" {
		watchConfig.Dir = strings.TrimRight(dirName, "/")
	}

	return watchConfig
}
