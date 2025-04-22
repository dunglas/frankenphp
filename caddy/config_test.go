package caddy

import (
	"testing"

	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/stretchr/testify/require"
)

// resetModuleWorkers resets the moduleWorkers slice for testing
func resetModuleWorkers() {
	moduleWorkers = make([]workerConfig, 0)
}

func TestModuleWorkerDuplicateFilenamesFail(t *testing.T) {
	// Create a test configuration with duplicate worker filenames
	configWithDuplicateFilenames := `
	{
		php {
			worker {
				file worker-with-env.php
				num 1
			}
			worker {
				file worker-with-env.php
				num 2
			}
		}
	}`

	// Parse the configuration
	d := caddyfile.NewTestDispenser(configWithDuplicateFilenames)
	module := &FrankenPHPModule{}

	// Unmarshal the configuration
	err := module.UnmarshalCaddyfile(d)

	// Verify that an error was returned
	require.Error(t, err, "Expected an error when two workers in the same module have the same filename")
	require.Contains(t, err.Error(), "workers must not have duplicate filenames", "Error message should mention duplicate filenames")
	resetModuleWorkers()
}

func TestModuleWorkersDuplicateNameFail(t *testing.T) {
	// Create a test configuration with a worker name
	configWithWorkerName1 := `
	{
		php_server {
			worker {
				name test-worker
				file ../testdata/worker-with-env.php
				num 1
			}
		}
	}`

	// Parse the first configuration
	d1 := caddyfile.NewTestDispenser(configWithWorkerName1)
	module1 := &FrankenPHPModule{}

	// Unmarshal the first configuration
	err := module1.UnmarshalCaddyfile(d1)
	require.NoError(t, err, "First module should be configured without errors")

	// Create a second test configuration with the same worker name
	configWithWorkerName2 := `
	{
		php_server {
			worker {
				name test-worker
				file ../testdata/worker-with-env.php
				num 1
			}
		}
	}`

	// Parse the second configuration
	d2 := caddyfile.NewTestDispenser(configWithWorkerName2)
	module2 := &FrankenPHPModule{}

	// Unmarshal the second configuration
	err = module2.UnmarshalCaddyfile(d2)

	// Verify that an error was returned
	require.Error(t, err, "Expected an error when two workers have the same name")
	require.Contains(t, err.Error(), "workers must not have duplicate names", "Error message should mention duplicate names")
	resetModuleWorkers()
}
