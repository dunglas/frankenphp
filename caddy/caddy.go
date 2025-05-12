// Package caddy provides a PHP module for the Caddy web server.
// FrankenPHP embeds the PHP interpreter directly in Caddy, giving it the ability to run your PHP scripts directly.
// No PHP FPM required!
package caddy

import (
	"fmt"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/caddyserver/caddy/v2/caddyconfig/httpcaddyfile"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
)

const (
	defaultDocumentRoot = "public"
	defaultWatchPattern = "./**/*.{php,yaml,yml,twig,env}"
)

// FrankenPHPModule instances register their workers, and FrankenPHPApp reads them at Start() time.
// FrankenPHPApp.Workers may be set by JSON config, so keep them separate.
var moduleWorkerConfigs []workerConfig

func init() {
	caddy.RegisterModule(FrankenPHPApp{})
	caddy.RegisterModule(FrankenPHPModule{})
	caddy.RegisterModule(FrankenPHPAdmin{})

	httpcaddyfile.RegisterGlobalOption("frankenphp", parseFrankenPhpDirective)

	httpcaddyfile.RegisterHandlerDirective("php", parsePhpDirective)
	httpcaddyfile.RegisterDirectiveOrder("php", "before", "file_server")

	httpcaddyfile.RegisterDirective("php_server", parsePhpServerDirective)
	httpcaddyfile.RegisterDirectiveOrder("php_server", "before", "file_server")

	httpcaddyfile.RegisterDirective("php_worker", parsePhpWorkerDirective)
	httpcaddyfile.RegisterDirectiveOrder("php_worker", "before", "file_server")
}

// return a nice error message
func wrongSubDirectiveError(module string, allowedDriectives string, wrongValue string) error {
	return fmt.Errorf("unknown '%s' subdirective: '%s' (allowed directives are: %s)", module, wrongValue, allowedDriectives)
}

// Interface guards
var (
	_ caddy.App                   = (*FrankenPHPApp)(nil)
	_ caddy.Provisioner           = (*FrankenPHPApp)(nil)
	_ caddy.Provisioner           = (*FrankenPHPModule)(nil)
	_ caddyhttp.MiddlewareHandler = (*FrankenPHPModule)(nil)
	_ caddyfile.Unmarshaler       = (*FrankenPHPModule)(nil)
)
