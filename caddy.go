// Package frankenphp provide a PHP module for Caddy.
// FrankenPHP embeds the PHP interpreter directly in Caddy, giving it the ability to run your PHP scripts directly.
// No PHP FPM required!
package frankenphp

import (
	"net/http"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/caddyserver/caddy/v2/caddyconfig/httpcaddyfile"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	"go.uber.org/zap"
)

func init() {
	caddy.RegisterModule(FrankenPHP{})
	httpcaddyfile.RegisterHandlerDirective("php", parseCaddyfile)
}

type FrankenPHP struct {
	logger *zap.Logger
}

// CaddyModule returns the Caddy module information.
func (FrankenPHP) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "http.handlers.php",
		New: func() caddy.Module { return new(FrankenPHP) },
	}
}

// Provision sets up the module.
func (f *FrankenPHP) Provision(ctx caddy.Context) error {
	f.logger = ctx.Logger(f)
	Startup()

	return nil
}

func (m *FrankenPHP) Cleanup() error {
	Shutdown()

	return nil
}

// ServeHTTP implements caddyhttp.MiddlewareHandler.
func (f FrankenPHP) ServeHTTP(w http.ResponseWriter, r *http.Request, next caddyhttp.Handler) error {
	w.Write([]byte("Hello World"))
	return nil
}

// UnmarshalCaddyfile implements caddyfile.Unmarshaler.
func (m *FrankenPHP) UnmarshalCaddyfile(d *caddyfile.Dispenser) error {
	return nil
}

// parseCaddyfile unmarshals tokens from h into a new Middleware.
func parseCaddyfile(h httpcaddyfile.Helper) (caddyhttp.MiddlewareHandler, error) {
	var m FrankenPHP
	err := m.UnmarshalCaddyfile(h.Dispenser)
	return m, err
}

// Interface guards
var (
	_ caddy.Provisioner           = (*FrankenPHP)(nil)
	_ caddy.CleanerUpper          = (*FrankenPHP)(nil)
	_ caddyhttp.MiddlewareHandler = (*FrankenPHP)(nil)
	_ caddyfile.Unmarshaler       = (*FrankenPHP)(nil)
)
