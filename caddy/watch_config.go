package caddy

import (
	"strconv"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/dunglas/frankenphp"
)

type watchConfig struct {
	// FileName sets the path to the worker script.
	Dirs []string `json:"dir,omitempty"`
	// Determines whether the watcher should be recursive.
	Recursive bool `json:"recursive,omitempty"`
	// Determines whether the watcher should follow symlinks.
    FollowSymlinks bool `json:"follow_symlinks,omitempty"`
	// Determines whether the regex should be case sensitive.
	CaseSensitive bool `json:"case_sensitive,omitempty"`
	// Determines whether the regex should be extended.
	ExtendedRegex bool `json:"extended_regex,omitempty"`
	// Latency of the watcher in ms.
	Latency int `json:"latency,omitempty"`
	// Include only files matching this regex
	IncludeFiles string `json:"include,omitempty"`
	// Exclude files matching this regex (will exclude all if empty)
	ExcludeFiles string `json:"exclude,omitempty"`
	// Allowed: "default", "fsevents", "kqueue", "inotify", "windows", "poll", "fen"
	MonitorType string `json:"monitor_type,omitempty"`
	// Use wildcard pattern instead of regex to match files
	WildcardPattern string `json:"wildcard_pattern,omitempty"`
	// Determines weather to use the one line short-form
	IsShortForm bool
}

func applyWatchConfig(opts []frankenphp.Option, watchConfig watchConfig) []frankenphp.Option {
	if watchConfig.IsShortForm {
		return append(opts, frankenphp.WithFileWatcher(
            frankenphp.WithWatcherShortForm(watchConfig.Dirs[0]),
            frankenphp.WithWatcherMonitorType(watchConfig.MonitorType),
		))
	}
	return append(opts, frankenphp.WithFileWatcher(
        frankenphp.WithWatcherDirs(watchConfig.Dirs),
        frankenphp.WithWatcherRecursion(watchConfig.Recursive),
        frankenphp.WithWatcherSymlinks(watchConfig.FollowSymlinks),
        frankenphp.WithWatcherFilters(watchConfig.IncludeFiles, watchConfig.ExcludeFiles, watchConfig.CaseSensitive, watchConfig.ExtendedRegex),
        frankenphp.WithWatcherLatency(watchConfig.Latency),
        frankenphp.WithWatcherMonitorType(watchConfig.MonitorType),
        frankenphp.WithWildcardPattern(watchConfig.WildcardPattern),
    ))
}

func parseWatchDirective(f *FrankenPHPApp, d *caddyfile.Dispenser) error {
	watchConfig := watchConfig{
		Recursive: true,
		Latency: 150,
	}
	if d.NextArg() {
		watchConfig.Dirs = append(watchConfig.Dirs, d.Val())
		watchConfig.IsShortForm = true
	}

	if d.NextArg() {
        if err := verifyMonitorType(d.Val(), d); err != nil {
            return err
        }
        watchConfig.MonitorType = d.Val()
    }

	for d.NextBlock(1) {
        v := d.Val()
        switch v {
		case "dir", "directory", "path":
            if !d.NextArg() {
                return d.ArgErr()
            }
            watchConfig.Dirs = append(watchConfig.Dirs, d.Val())
        case "recursive":
            if !d.NextArg() {
                watchConfig.Recursive = true
                continue
            }
            v, err := strconv.ParseBool(d.Val())
            if err != nil {
                return err
            }
            watchConfig.Recursive = v
        case "follow_symlinks", "symlinks":
            if !d.NextArg() {
                watchConfig.FollowSymlinks = true
                continue
            }
            v, err := strconv.ParseBool(d.Val())
            if err != nil {
                return err
            }
            watchConfig.FollowSymlinks = v
        case "latency":
            if !d.NextArg() {
                return d.ArgErr()
            }
            v, err := strconv.Atoi(d.Val())
            if err != nil {
                return err
            }
            watchConfig.Latency = v
		case "include", "include_files":
			if !d.NextArg() {
				return d.ArgErr()
			}
			watchConfig.IncludeFiles = d.Val()
		case "exclude", "exclude_files":
			if !d.NextArg() {
				return d.ArgErr()
			}
			watchConfig.ExcludeFiles = d.Val()
		case "case_sensitive":
			if !d.NextArg() {
				watchConfig.CaseSensitive = true
				continue
			}
			v, err := strconv.ParseBool(d.Val())
			if err != nil {
				return err
			}
			watchConfig.CaseSensitive = v
		case "pattern", "wildcard":
			if !d.NextArg() {
				return d.ArgErr()
				continue
			}
			watchConfig.WildcardPattern = d.Val()
		case "extended_regex":
			if !d.NextArg() {
				watchConfig.ExtendedRegex = true
				continue
			}
			v, err := strconv.ParseBool(d.Val())
			if err != nil {
				return err
			}
			watchConfig.ExtendedRegex = v
		case "monitor_type", "monitor":
			if !d.NextArg() {
				return d.ArgErr()
			}
			if err := verifyMonitorType(d.Val(), d); err != nil {
				return err
			}
			watchConfig.MonitorType = d.Val()
		default:
			return d.Errf("unknown watcher subdirective '%s'", v)
		}
    }
	if(len(watchConfig.Dirs) == 0) {
		return d.Err("The 'dir' argument must be specified for the watch directive")
	}
	f.Watch = append(f.Watch, watchConfig)
	return nil
}

func verifyMonitorType(monitorType string, d *caddyfile.Dispenser) error {
	switch monitorType {
        case "default", "system", "fsevents", "kqueue", "inotify", "windows", "poll", "fen":
            return nil
        default:
            return d.Errf("unknown watcher monitor type '%s'", monitorType)
    }
}