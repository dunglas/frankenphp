package caddy

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/caddyserver/caddy/v2"
	caddycmd "github.com/caddyserver/caddy/v2/cmd"
	"github.com/dunglas/frankenphp"
	"go.uber.org/zap/exp/zapslog"

	"github.com/spf13/cobra"
)

func init() {
	caddycmd.RegisterCommand(caddycmd.Command{
		Name:  "php-cli",
		Usage: "script.php [args ...]",
		Short: "Runs a PHP command",
		Long: `
Executes a PHP script similarly to the CLI SAPI.`,
		CobraFunc: func(cmd *cobra.Command) {
			cmd.DisableFlagParsing = true
			cmd.RunE = caddycmd.WrapCommandFuncForCobra(cmdPHPCLI)
		},
	})
}

func cmdPHPCLI(fs caddycmd.Flags) (int, error) {
	// php's cli sapi expects the 0th arg to be the program itself, only filter out 'php-cli' arg
	args := append([]string{os.Args[0]}, os.Args[2:]...)

	if frankenphp.EmbeddedAppPath != "" && len(args) > 1 && !strings.HasPrefix(args[1], "-") && strings.HasSuffix(args[1], ".php") {
		if _, err := os.Stat(args[1]); err != nil {
			args[1] = filepath.Join(frankenphp.EmbeddedAppPath, args[1])
		}
	}

	logger := slog.New(zapslog.NewHandler(caddy.Log().Core(), zapslog.WithName("frankenphp")))
	status := frankenphp.ExecuteScriptCLI(args[0], args, frankenphp.WithLogger(logger))

	os.Exit(status)

	return status, nil
}
