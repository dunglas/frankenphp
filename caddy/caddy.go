// Package frankenphp provides a PHP module for Caddy.
// FrankenPHP embeds the PHP interpreter directly in Caddy, giving it the ability to run your PHP scripts directly.
// No PHP FPM required!
package caddy

import (
	"bytes"
	"log"
	"net/http"
	"runtime"
	"strconv"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/caddyserver/caddy/v2/caddyconfig/httpcaddyfile"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	"github.com/dunglas/frankenphp"
	"go.uber.org/zap"
)

func init() {
	frankenphp.Startup()

	caddy.RegisterModule(&FrankenPHPApp{})
	caddy.RegisterModule(FrankenPHPModule{})
	httpcaddyfile.RegisterGlobalOption("frankenphp", parseGlobalOption)
	httpcaddyfile.RegisterHandlerDirective("php", parseCaddyfile)
}

type FrankenPHPApp struct{}

// CaddyModule returns the Caddy module information.
func (a *FrankenPHPApp) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "frankenphp",
		New: func() caddy.Module { return a },
	}
}

func getGID() uint64 {
	b := make([]byte, 64)
	b = b[:runtime.Stack(b, false)]
	b = bytes.TrimPrefix(b, []byte("goroutine "))
	b = b[:bytes.IndexByte(b, ' ')]
	n, _ := strconv.ParseUint(string(b), 10, 64)
	return n
}

func (*FrankenPHPApp) Start() error {
	log.Printf("started! %d", getGID())
	return frankenphp.Startup()
}

func (*FrankenPHPApp) Stop() error {
	log.Printf("stoped!")

	frankenphp.Shutdown()

	return nil
}

func parseGlobalOption(d *caddyfile.Dispenser, _ interface{}) (interface{}, error) {
	app := &FrankenPHPApp{}

	// tell Caddyfile adapter that this is the JSON for an app
	return httpcaddyfile.App{
		Name:  "frankenphp",
		Value: caddyconfig.JSON(app, nil),
	}, nil
}

type FrankenPHPModule struct {
	Root               string            `json:"root,omitempty"`
	SplitPath          []string          `json:"split_path,omitempty"`
	ResolveRootSymlink bool              `json:"resolve_root_symlink,omitempty"`
	Env                map[string]string `json:"env,omitempty"`
	logger             *zap.Logger
}

// CaddyModule returns the Caddy module information.
func (FrankenPHPModule) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "http.handlers.php",
		New: func() caddy.Module { return new(FrankenPHPModule) },
	}
}

// Provision sets up the module.
func (f *FrankenPHPModule) Provision(ctx caddy.Context) error {
	f.logger = ctx.Logger(f)

	if f.Root == "" {
		f.Root = "{http.vars.root}"
	}
	if len(f.SplitPath) == 0 {
		f.SplitPath = []string{".php"}
	}

	return nil
}

// ServeHTTP implements caddyhttp.MiddlewareHandler.
// TODO: Expose TLS versions as env vars, as Apache's mod_ssl: https://github.com/caddyserver/caddy/blob/master/modules/caddyhttp/reverseproxy/fastcgi/fastcgi.go#L298
func (f FrankenPHPModule) ServeHTTP(w http.ResponseWriter, r *http.Request, next caddyhttp.Handler) error {
	origReq := r.Context().Value(caddyhttp.OriginalRequestCtxKey).(http.Request)
	repl := r.Context().Value(caddy.ReplacerCtxKey).(*caddy.Replacer)

	documentRoot := repl.ReplaceKnown(f.Root, "")
	fr := frankenphp.NewRequestWithContext(r, documentRoot)
	fc, _ := frankenphp.FromContext(fr.Context())
	fc.ResolveRootSymlink = f.ResolveRootSymlink
	fc.SplitPath = f.SplitPath

	fc.Env["REQUEST_URI"] = origReq.URL.RequestURI()
	for k, v := range f.Env {
		fc.Env[k] = repl.ReplaceKnown(v, "")
	}

	if err := frankenphp.ExecuteScript(w, fr); err != nil {
		return err
	}

	return nil
}

// UnmarshalCaddyfile implements caddyfile.Unmarshaler.
func (f *FrankenPHPModule) UnmarshalCaddyfile(d *caddyfile.Dispenser) error {
	for d.Next() {
		for d.NextBlock(0) {
			switch d.Val() {
			case "root":
				if !d.NextArg() {
					return d.ArgErr()
				}
				f.Root = d.Val()

			case "split":
				f.SplitPath = d.RemainingArgs()
				if len(f.SplitPath) == 0 {
					return d.ArgErr()
				}

			case "env":
				args := d.RemainingArgs()
				if len(args) != 2 {
					return d.ArgErr()
				}
				if f.Env == nil {
					f.Env = make(map[string]string)
				}
				f.Env[args[0]] = args[1]

			case "resolve_root_symlink":
				if d.NextArg() {
					return d.ArgErr()
				}
				f.ResolveRootSymlink = true
			}
		}
	}

	return nil
}

// parseCaddyfile unmarshals tokens from h into a new Middleware.
func parseCaddyfile(h httpcaddyfile.Helper) (caddyhttp.MiddlewareHandler, error) {
	var m FrankenPHPModule
	err := m.UnmarshalCaddyfile(h.Dispenser)

	return m, err
}

// Interface guards
var (
	_ caddy.App                   = (*FrankenPHPApp)(nil)
	_ caddy.Provisioner           = (*FrankenPHPModule)(nil)
	_ caddyhttp.MiddlewareHandler = (*FrankenPHPModule)(nil)
	_ caddyfile.Unmarshaler       = (*FrankenPHPModule)(nil)
)
