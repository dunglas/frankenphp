//go:build !include_php_cli

package main

import caddycmd "github.com/caddyserver/caddy/v2/cmd"

func runPhpCli() {
	caddycmd.Main()
}
