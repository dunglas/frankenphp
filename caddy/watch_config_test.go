package caddy_test

import (
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/dunglas/frankenphp/caddy"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestParsingARecursiveDirectory(t *testing.T) {
	app, err := parseTestConfig(`
		frankenphp {
			watch /path
		}
	`)

	assert.Nil(t, err)
	assert.Equal(t, 1, len(app.Watch))
	assert.Equal(t, 1, len(app.Watch[0].Dirs))
	assert.Equal(t, "/path", app.Watch[0].Dirs[0])
	assert.True(t, app.Watch[0].IsRecursive)
	assert.Equal(t, "", app.Watch[0].Pattern)
}

func TestParsingARecursiveDirectoryWithPattern(t *testing.T) {
	app, err := parseTestConfig(`
		frankenphp {
			watch /path/**/*.php
		}
	`)

	assert.Nil(t, err)
	assert.Equal(t, 1, len(app.Watch))
	assert.Equal(t, "/path", app.Watch[0].Dirs[0])
	assert.True(t, app.Watch[0].IsRecursive)
	assert.Equal(t, "*.php", app.Watch[0].Pattern)
}

func TestParsingNonRecursiveDirectoryWithPattern(t *testing.T) {
	app, err := parseTestConfig(`
		frankenphp {
			watch /path/*.php
		}
	`)

	assert.Nil(t, err)
	assert.Equal(t, 1, len(app.Watch))
	assert.Equal(t, "/path", app.Watch[0].Dirs[0])
	assert.False(t, app.Watch[0].IsRecursive)
	assert.Equal(t, "*.php", app.Watch[0].Pattern)
}

func TestParseTwoShortForms(t *testing.T) {
	app, err := parseTestConfig(`
		frankenphp {
			watch /path
			watch /other/path/*.php
		}
	`)

	assert.Nil(t, err)
	assert.Equal(t, 2, len(app.Watch))
	assert.Equal(t, "/path", app.Watch[0].Dirs[0])
	assert.Equal(t, "", app.Watch[0].Pattern)
	assert.Equal(t, "/other/path", app.Watch[1].Dirs[0])
	assert.Equal(t, "*.php", app.Watch[1].Pattern)
}

func TestFailOnMissingPath(t *testing.T) {
	_, err := parseTestConfig(`
		frankenphp {
			watch
		}
	`)

	assert.Error(t, err)
}

func parseTestConfig(config string) (*caddy.FrankenPHPApp, error) {
	app := caddy.FrankenPHPApp{}
	err := app.UnmarshalCaddyfile(caddyfile.NewTestDispenser(config))
	return &app, err
}
