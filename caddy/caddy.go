// Package frankenphp provides a PHP module for Caddy.
// FrankenPHP embeds the PHP interpreter directly in Caddy, giving it the ability to run your PHP scripts directly.
// No PHP FPM required!
package caddy

import (
	"net/http"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/caddyserver/caddy/v2/caddyconfig/httpcaddyfile"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	"github.com/dunglas/frankenphp"
	"go.uber.org/zap"
)

var php = caddy.NewUsagePool()

func init() {
	caddy.RegisterModule(FrankenPHPModule{})
	httpcaddyfile.RegisterHandlerDirective("php", parseCaddyfile)
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

type phpDestructor struct{}

func (phpDestructor) Destruct() error {
	frankenphp.Shutdown()

	return nil
}

// Provision sets up the module.
func (f *FrankenPHPModule) Provision(ctx caddy.Context) error {
	f.logger = ctx.Logger(f)

	_, _, err := php.LoadOrNew("php", func() (caddy.Destructor, error) {
		frankenphp.Startup()
		return &phpDestructor{}, nil
	})
	if err != nil {
		return err //nolint:wrapcheck
	}

	if f.Root == "" {
		f.Root = "{http.vars.root}"
	}
	if len(f.SplitPath) == 0 {
		f.SplitPath = []string{".php"}
	}

	return nil
}

func (f *FrankenPHPModule) Cleanup() error {
	_, err := php.Delete("php")

	return err
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
	_ caddy.Provisioner           = (*FrankenPHPModule)(nil)
	_ caddy.CleanerUpper          = (*FrankenPHPModule)(nil)
	_ caddyhttp.MiddlewareHandler = (*FrankenPHPModule)(nil)
	_ caddyfile.Unmarshaler       = (*FrankenPHPModule)(nil)
)
