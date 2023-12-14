package main

import (
	"github.com/caddyserver/caddy/v2"
	caddycmd "github.com/caddyserver/caddy/v2/cmd"
	"go.uber.org/automaxprocs/maxprocs"
	"go.uber.org/zap"

	// plug in Caddy modules here.
	_ "github.com/caddyserver/caddy/v2/modules/standard"
	_ "github.com/dunglas/frankenphp/caddy"
	_ "github.com/dunglas/mercure/caddy"
	_ "github.com/dunglas/vulcain/caddy"
)

func main() {
	undo, err := maxprocs.Set()
	defer undo()
	if err != nil {
		caddy.Log().Warn("failed to set GOMAXPROCS", zap.Error(err))
	}

	caddycmd.Main()
}
