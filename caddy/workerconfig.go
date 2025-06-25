package caddy

import (
	"errors"
	"net/http"
	"path/filepath"
	"strconv"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	"github.com/dunglas/frankenphp"
	"github.com/dunglas/frankenphp/internal/fastabs"
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
	// The path to match against the worker
	MatchPath []string `json:"match_path,omitempty"`
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
		case "match":
			// provision the path so it's identical to Caddy match rules
			// see: https://github.com/caddyserver/caddy/blob/master/modules/caddyhttp/matchers.go
			caddyMatchPath := (caddyhttp.MatchPath)(d.RemainingArgs())
			caddyMatchPath.Provision(caddy.Context{})
			wc.MatchPath = ([]string)(caddyMatchPath)
		default:
			allowedDirectives := "name, file, num, env, watch, match"
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

func (wc workerConfig) inheritEnv(env map[string]string) {
	if wc.Env == nil {
		wc.Env = make(map[string]string)
	}
	for k, v := range env {
		// do not overwrite existing environment variables
		if _, exists := wc.Env[k]; !exists {
			wc.Env[k] = v
		}
	}
}

func (wc workerConfig) matchesPath(r *http.Request, documentRoot string) bool {

	// try to match against a pattern if one is assigned
	if len(wc.MatchPath) != 0 {
		return (caddyhttp.MatchPath)(wc.MatchPath).Match(r)
	}

	// if there is no pattern, try to match against the actual path (in the public directory)
	fullScriptPath, _ := fastabs.FastAbs(documentRoot + "/" + r.URL.Path)
	absFileName, _ := fastabs.FastAbs(wc.FileName)

	return fullScriptPath == absFileName
}
