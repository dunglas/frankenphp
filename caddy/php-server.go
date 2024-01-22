package caddy

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig"
	caddycmd "github.com/caddyserver/caddy/v2/cmd"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp/encode"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp/fileserver"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp/rewrite"
	"github.com/caddyserver/certmagic"
	"github.com/dunglas/frankenphp"
	"go.uber.org/zap"

	"github.com/spf13/cobra"
)

func init() {
	caddycmd.RegisterCommand(caddycmd.Command{
		Name:  "php-server",
		Usage: "[--domain <example.com>] [--root <path>] [--listen <addr>] [--worker /path/to/worker.php<,nb-workers>] [--access-log] [--debug] [--no-compress]",
		Short: "Spins up a production-ready PHP server",
		Long: `
A simple but production-ready PHP server. Useful for quick deployments,
demos, and development.

The listener's socket address can be customized with the --listen flag.

If a domain name is specified with --domain, the default listener address
will be changed to the HTTPS port and the server will use HTTPS. If using
a public domain, ensure A/AAAA records are properly configured before
using this option.

For more advanced use cases, see https://github.com/dunglas/frankenphp/blob/main/docs/config.md`,
		CobraFunc: func(cmd *cobra.Command) {
			cmd.Flags().StringP("domain", "d", "", "Domain name at which to serve the files")
			cmd.Flags().StringP("root", "r", "", "The path to the root of the site")
			cmd.Flags().StringP("listen", "l", "", "The address to which to bind the listener")
			cmd.Flags().StringArrayP("worker", "w", []string{}, "Worker script")
			cmd.Flags().BoolP("access-log", "a", false, "Enable the access log")
			cmd.Flags().BoolP("debug", "v", false, "Enable verbose debug logs")
			cmd.Flags().BoolP("no-compress", "", false, "Disable Zstandard and Gzip compression")
			cmd.RunE = caddycmd.WrapCommandFuncForCobra(cmdPHPServer)
		},
	})
}

// cmdPHPServer is freely inspired from the file-server command of the Caddy server (Apache License 2.0, Matthew Holt and The Caddy Authors)
func cmdPHPServer(fs caddycmd.Flags) (int, error) {
	caddy.TrapSignals()

	domain := fs.String("domain")
	root := fs.String("root")
	listen := fs.String("listen")
	accessLog := fs.Bool("access-log")
	debug := fs.Bool("debug")
	compress := !fs.Bool("no-compress")

	workers, err := fs.GetStringArray("worker")
	if err != nil {
		panic(err)
	}

	var workersOption []workerConfig
	if len(workers) != 0 {
		workersOption = make([]workerConfig, 0, len(workers))
		for _, worker := range workers {
			parts := strings.SplitN(worker, ",", 2)
			if frankenphp.EmbeddedAppPath != "" && filepath.IsLocal(parts[0]) {
				parts[0] = filepath.Join(frankenphp.EmbeddedAppPath, parts[0])
			}

			var num int
			if len(parts) > 1 {
				num, _ = strconv.Atoi(parts[1])
			}

			workersOption = append(workersOption, workerConfig{FileName: parts[0], Num: num})
		}
	}

	if frankenphp.EmbeddedAppPath != "" {
		if _, err := os.Stat(filepath.Join(frankenphp.EmbeddedAppPath, "Caddyfile")); err == nil {
			config, _, err := caddycmd.LoadConfig(filepath.Join(frankenphp.EmbeddedAppPath, "Caddyfile"), "")
			if err != nil {
				return caddy.ExitCodeFailedStartup, err
			}

			if err = caddy.Load(config, true); err != nil {
				return caddy.ExitCodeFailedStartup, err
			}

			select {}
		}

		if root == "" {
			root = filepath.Join(frankenphp.EmbeddedAppPath, defaultDocumentRoot)
		} else if filepath.IsLocal(root) {
			root = filepath.Join(frankenphp.EmbeddedAppPath, root)
		}
	}

	const indexFile = "index.php"
	extensions := []string{"php"}
	tryFiles := []string{"{http.request.uri.path}", "{http.request.uri.path}/" + indexFile, indexFile}

	phpHandler := FrankenPHPModule{
		Root:      root,
		SplitPath: extensions,
	}

	// route to redirect to canonical path if index PHP file
	redirMatcherSet := caddy.ModuleMap{
		"file": caddyconfig.JSON(fileserver.MatchFile{
			Root:     root,
			TryFiles: []string{"{http.request.uri.path}/" + indexFile},
		}, nil),
		"not": caddyconfig.JSON(caddyhttp.MatchNot{
			MatcherSetsRaw: []caddy.ModuleMap{
				{
					"path": caddyconfig.JSON(caddyhttp.MatchPath{"*/"}, nil),
				},
			},
		}, nil),
	}
	redirHandler := caddyhttp.StaticResponse{
		StatusCode: caddyhttp.WeakString(strconv.Itoa(http.StatusPermanentRedirect)),
		Headers:    http.Header{"Location": []string{"{http.request.orig_uri.path}/"}},
	}
	redirRoute := caddyhttp.Route{
		MatcherSetsRaw: []caddy.ModuleMap{redirMatcherSet},
		HandlersRaw:    []json.RawMessage{caddyconfig.JSONModuleObject(redirHandler, "handler", "static_response", nil)},
	}

	// route to rewrite to PHP index file
	rewriteMatcherSet := caddy.ModuleMap{
		"file": caddyconfig.JSON(fileserver.MatchFile{
			Root:      root,
			TryFiles:  tryFiles,
			SplitPath: extensions,
		}, nil),
	}
	rewriteHandler := rewrite.Rewrite{
		URI: "{http.matchers.file.relative}",
	}
	rewriteRoute := caddyhttp.Route{
		MatcherSetsRaw: []caddy.ModuleMap{rewriteMatcherSet},
		HandlersRaw:    []json.RawMessage{caddyconfig.JSONModuleObject(rewriteHandler, "handler", "rewrite", nil)},
	}

	// route to actually pass requests to PHP files;
	// match only requests that are for PHP files
	pathList := []string{}
	for _, ext := range extensions {
		pathList = append(pathList, "*"+ext)
	}
	phpMatcherSet := caddy.ModuleMap{
		"path": caddyconfig.JSON(pathList, nil),
	}

	// create the PHP route which is
	// conditional on matching PHP files
	phpRoute := caddyhttp.Route{
		MatcherSetsRaw: []caddy.ModuleMap{phpMatcherSet},
		HandlersRaw:    []json.RawMessage{caddyconfig.JSONModuleObject(phpHandler, "handler", "php", nil)},
	}

	fileRoute := caddyhttp.Route{
		MatcherSetsRaw: []caddy.ModuleMap{},
		HandlersRaw:    []json.RawMessage{caddyconfig.JSONModuleObject(fileserver.FileServer{Root: root}, "handler", "file_server", nil)},
	}

	subroute := caddyhttp.Subroute{
		Routes: caddyhttp.RouteList{redirRoute, rewriteRoute, phpRoute, fileRoute},
	}

	if compress {
		gzip, err := caddy.GetModule("http.encoders.gzip")
		if err != nil {
			return caddy.ExitCodeFailedStartup, err
		}

		zstd, err := caddy.GetModule("http.encoders.zstd")
		if err != nil {
			return caddy.ExitCodeFailedStartup, err
		}

		encodeRoute := caddyhttp.Route{
			MatcherSetsRaw: []caddy.ModuleMap{},
			HandlersRaw: []json.RawMessage{caddyconfig.JSONModuleObject(encode.Encode{
				EncodingsRaw: caddy.ModuleMap{
					"zstd": caddyconfig.JSON(zstd.New(), nil),
					"gzip": caddyconfig.JSON(gzip.New(), nil),
				},
				Prefer: []string{"zstd", "gzip"},
			}, "handler", "encode", nil)},
		}

		subroute.Routes = append(caddyhttp.RouteList{encodeRoute}, subroute.Routes...)
	}

	route := caddyhttp.Route{
		HandlersRaw: []json.RawMessage{caddyconfig.JSONModuleObject(subroute, "handler", "subroute", nil)},
	}

	if domain != "" {
		route.MatcherSetsRaw = []caddy.ModuleMap{
			{
				"host": caddyconfig.JSON(caddyhttp.MatchHost{domain}, nil),
			},
		}
	}

	server := &caddyhttp.Server{
		ReadHeaderTimeout: caddy.Duration(10 * time.Second),
		IdleTimeout:       caddy.Duration(30 * time.Second),
		MaxHeaderBytes:    1024 * 10,
		Routes:            caddyhttp.RouteList{route},
	}
	if listen == "" {
		if domain == "" {
			listen = ":80"
		} else {
			listen = ":" + strconv.Itoa(certmagic.HTTPSPort)
		}
	}
	server.Listen = []string{listen}
	if accessLog {
		server.Logs = &caddyhttp.ServerLogConfig{}
	}

	httpApp := caddyhttp.App{
		Servers: map[string]*caddyhttp.Server{"php": server},
	}

	var false bool
	cfg := &caddy.Config{
		Admin: &caddy.AdminConfig{
			Disabled: true,
			Config: &caddy.ConfigSettings{
				Persist: &false,
			},
		},
		AppsRaw: caddy.ModuleMap{
			"http":       caddyconfig.JSON(httpApp, nil),
			"frankenphp": caddyconfig.JSON(FrankenPHPApp{Workers: workersOption}, nil),
		},
	}

	if debug {
		cfg.Logging = &caddy.Logging{
			Logs: map[string]*caddy.CustomLog{
				"default": {
					BaseLog: caddy.BaseLog{Level: zap.DebugLevel.CapitalString()},
				},
			},
		}
	}

	err = caddy.Run(cfg)
	if err != nil {
		return caddy.ExitCodeFailedStartup, err
	}

	log.Printf("Caddy serving PHP app on %s", listen)

	select {}
}
