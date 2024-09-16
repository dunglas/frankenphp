package caddy_test

import (
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/dunglas/frankenphp/caddy"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestParseRecursiveDirectoryWithoutPattern(t *testing.T) {
	app, err := parseTestConfig(`
		frankenphp {
			watch /path1
			watch /path2/
			watch /path3/**/
		}
	`)

	assert.Nil(t, err)
	assert.Equal(t, 3, len(app.Watch))

	assert.Equal(t, "/path1", app.Watch[0].Dir)
	assert.Equal(t, "/path2", app.Watch[1].Dir)
	assert.Equal(t, "/path3", app.Watch[2].Dir)
	assert.True(t, app.Watch[0].IsRecursive)
	assert.True(t, app.Watch[1].IsRecursive)
	assert.True(t, app.Watch[2].IsRecursive)
	assert.Equal(t, "", app.Watch[0].Pattern)
	assert.Equal(t, "", app.Watch[1].Pattern)
	assert.Equal(t, "", app.Watch[2].Pattern)
}

func TestParseRecursiveDirectoryWithPattern(t *testing.T) {
	app, err := parseTestConfig(`
		frankenphp {
			watch /path/**/*.php
			watch /path/**/filename
		}
	`)

	assert.Nil(t, err)
	assert.Equal(t, 2, len(app.Watch))
	assert.Equal(t, "/path", app.Watch[0].Dir)
	assert.Equal(t, "/path", app.Watch[1].Dir)
	assert.True(t, app.Watch[0].IsRecursive)
	assert.True(t, app.Watch[1].IsRecursive)
	assert.Equal(t, "*.php", app.Watch[0].Pattern)
	assert.Equal(t, "filename", app.Watch[1].Pattern)
}

func TestParseNonRecursiveDirectoryWithPattern(t *testing.T) {
	app, err := parseTestConfig(`
		frankenphp {
			watch /path1/*.php
			watch /path2/watch-me.php
		}
	`)

	assert.Nil(t, err)
	assert.Equal(t, 2, len(app.Watch))
	assert.Equal(t, "/path1", app.Watch[0].Dir)
	assert.Equal(t, "/path2", app.Watch[1].Dir)
	assert.False(t, app.Watch[0].IsRecursive)
	assert.False(t, app.Watch[1].IsRecursive)
	assert.Equal(t, "*.php", app.Watch[0].Pattern)
	assert.Equal(t, "watch-me.php", app.Watch[1].Pattern)
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
