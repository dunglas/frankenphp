package caddy

import (
	"errors"
	"os"

	caddycmd "github.com/caddyserver/caddy/v2/cmd"
	"github.com/dunglas/frankenphp"

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
			cmd.RunE = caddycmd.WrapCommandFuncForCobra(cmdPHPCLI)
		},
	})
}

func cmdPHPCLI(fs caddycmd.Flags) (int, error) {
	args := fs.Args()
	if len(args) < 1 {
		return 1, errors.New("the path to the PHP script is required")
	}

	status := frankenphp.ExecuteScriptCLI(args[0], args)
	os.Exit(status)

	return status, nil
}
