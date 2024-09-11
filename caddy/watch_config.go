package caddy

import (
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/dunglas/frankenphp"
	"path/filepath"
	"strings"
)

type watchConfig struct {
	// FileName sets the path to the worker script.
	Dirs []string `json:"dir,omitempty"`
	// Whether to watch the directory recursively
	IsRecursive bool `json:"recursive,omitempty"`
	// The shell filename pattern to match against
	Pattern string `json:"pattern,omitempty"`
}

func applyWatchConfig(opts []frankenphp.Option, watchConfig watchConfig) []frankenphp.Option {
	return append(opts, frankenphp.WithFileWatcher(
		frankenphp.WithWatcherDirs(watchConfig.Dirs),
		frankenphp.WithWatcherRecursion(watchConfig.IsRecursive),
		frankenphp.WithWatcherPattern(watchConfig.Pattern),
	))
}

func parseWatchDirective(f *FrankenPHPApp, d *caddyfile.Dispenser) error {
	if !d.NextArg() {
		return d.Err("The 'watch' directive must be followed by a path")
	}
	f.Watch = append(f.Watch, parseFullPattern(d.Val()))

	return nil
}

// TODO: better path validation?
// for the one line short-form in the caddy config, aka: 'watch /path/*pattern'
func parseFullPattern(filePattern string) watchConfig {
	watchConfig := watchConfig{IsRecursive: true}
	dirName := filePattern
	splitDirName, baseName := filepath.Split(filePattern)
	if filePattern != "." && filePattern != ".." && strings.ContainsAny(baseName, "*.[?\\") {
		dirName = splitDirName
		watchConfig.Pattern = baseName
		watchConfig.IsRecursive = false
	}

	if strings.Contains(filePattern, "/**/") {
		dirName = strings.Split(filePattern, "/**/")[0]
		watchConfig.Pattern = strings.Split(filePattern, "/**/")[1]
		watchConfig.IsRecursive = true
	}
	dirName = strings.TrimRight(dirName, "/")
	watchConfig.Dirs = []string{dirName}

	return watchConfig
}
