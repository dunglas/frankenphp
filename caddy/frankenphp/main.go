package main

import (
	caddycmd "github.com/caddyserver/caddy/v2/cmd"
	"os"
	"path/filepath"

	// plug in Caddy modules here.
	_ "github.com/caddyserver/caddy/v2/modules/standard"
	_ "github.com/dunglas/caddy-cbrotli"
	_ "github.com/dunglas/frankenphp/caddy"
	_ "github.com/dunglas/mercure/caddy"
	_ "github.com/dunglas/vulcain/caddy"
)

func main() {
	// if the name of the current program is `php`, then call into the php-cli currently embedded
	programName := filepath.Base(os.Args[0])
	if programName == "php" {
		runPhpCli() // never returns for static builds with INCLUDE_CLI set
	}
	caddycmd.Main()
}
