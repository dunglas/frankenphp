package frankenphp_test

import (
	"testing"

	"github.com/dunglas/frankenphp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStartup(t *testing.T) {
	defer frankenphp.Shutdown()
	assert.Nil(t, frankenphp.Startup())
}

func TestExecuteString(t *testing.T) {
	defer frankenphp.Shutdown()
	require.Nil(t, frankenphp.Startup())

	assert.Nil(t, frankenphp.ExecuteScript("testdata/index.php"))
}
