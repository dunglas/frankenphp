package caddy

import (
	"errors"
	"path/filepath"
	"strconv"

	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/dunglas/frankenphp"
)

// workerConfig represents the "worker" directive in the Caddyfile
// it can appear in the "frankenphp", "php_server" and "php" directives
//
//	frankenphp {
//		worker {
//			name "my-worker"
//			file "my-worker.php"
//		}
//	}
type workerConfig struct {
	// Name for the worker. Default: the filename for FrankenPHPApp workers, always prefixed with "m#" for FrankenPHPModule workers.
	Name string `json:"name,omitempty"`
	// FileName sets the path to the worker script.
	FileName string `json:"file_name,omitempty"`
	// Num sets the number of workers to start.
	Num int `json:"num,omitempty"`
	// Env sets an extra environment variable to the given value. Can be specified more than once for multiple environment variables.
	Env map[string]string `json:"env,omitempty"`
	// Directories to watch for file changes
	Watch []string `json:"watch,omitempty"`
}

func parseWorkerConfig(d *caddyfile.Dispenser) (workerConfig, error) {
	wc := workerConfig{}
	if d.NextArg() {
		wc.FileName = d.Val()
	}

	if d.NextArg() {
		if d.Val() == "watch" {
			wc.Watch = append(wc.Watch, defaultWatchPattern)
		} else {
			v, err := strconv.ParseUint(d.Val(), 10, 32)
			if err != nil {
				return wc, err
			}

			wc.Num = int(v)
		}
	}

	if d.NextArg() {
		return wc, errors.New(`FrankenPHP: too many "worker" arguments: ` + d.Val())
	}

	for d.NextBlock(1) {
		v := d.Val()
		switch v {
		case "name":
			if !d.NextArg() {
				return wc, d.ArgErr()
			}
			wc.Name = d.Val()
		case "file":
			if !d.NextArg() {
				return wc, d.ArgErr()
			}
			wc.FileName = d.Val()
		case "num":
			if !d.NextArg() {
				return wc, d.ArgErr()
			}

			v, err := strconv.ParseUint(d.Val(), 10, 32)
			if err != nil {
				return wc, err
			}

			wc.Num = int(v)
		case "env":
			args := d.RemainingArgs()
			if len(args) != 2 {
				return wc, d.ArgErr()
			}
			if wc.Env == nil {
				wc.Env = make(map[string]string)
			}
			wc.Env[args[0]] = args[1]
		case "watch":
			if !d.NextArg() {
				// the default if the watch directory is left empty:
				wc.Watch = append(wc.Watch, defaultWatchPattern)
			} else {
				wc.Watch = append(wc.Watch, d.Val())
			}
		default:
			allowedDirectives := "name, file, num, env, watch"
			return wc, wrongSubDirectiveError("worker", allowedDirectives, v)
		}
	}

	if wc.FileName == "" {
		return wc, errors.New(`the "file" argument must be specified`)
	}

	if frankenphp.EmbeddedAppPath != "" && filepath.IsLocal(wc.FileName) {
		wc.FileName = filepath.Join(frankenphp.EmbeddedAppPath, wc.FileName)
	}

	return wc, nil
}
