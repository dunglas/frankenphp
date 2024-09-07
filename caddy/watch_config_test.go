package caddy_test

import (
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/dunglas/frankenphp/caddy"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestParsingARecursiveShortForm(t *testing.T) {
	app, err := parseTestConfig(`
		frankenphp {
			watch /path
		}
	`)

	assert.Nil(t, err)
	assert.Equal(t, 1, len(app.Watch))
	assert.Equal(t, 1, len(app.Watch[0].Dirs))
	assert.Equal(t, "/path", app.Watch[0].Dirs[0])
	assert.True(t, app.Watch[0].Recursive)
	assert.True(t, app.Watch[0].IsShortForm)
}

func TestParseTwoShortForms(t *testing.T) {
	app, err := parseTestConfig(`
		frankenphp {
			watch /path
			watch /other/path poll
		}
	`)

	assert.Nil(t, err)
	assert.Equal(t, 2, len(app.Watch))
	assert.Equal(t, "/path", app.Watch[0].Dirs[0])
	assert.Equal(t, "", app.Watch[0].MonitorType)
	assert.Equal(t, "/other/path", app.Watch[1].Dirs[0])
	assert.Equal(t, "poll", app.Watch[1].MonitorType)
}

func TestFailOnInvalidMonitorType(t *testing.T) {
	_, err := parseTestConfig(`
		frankenphp {
			watch /path invalid_monitor
		}
	`)

	assert.Error(t, err)
}

func TestFailOnMissingPathInShortForm(t *testing.T) {
	_, err := parseTestConfig(`
		frankenphp {
			watch
		}
	`)

	assert.Error(t, err)
}

func TestFailOnMissingPathInLongForm(t *testing.T) {
	_, err := parseTestConfig(`
		frankenphp {
			watch {
				monitor_type poll
			}
		}
	`)

	assert.Error(t, err)
}

func TestParseLongFormCorrectly(t *testing.T) {
	app, err := parseTestConfig(`
		frankenphp {
			watch {
				path /path
				recursive false
				symlinks true
				case_sensitive true
				extended_regex true
				include *important.txt
				exclude *.txt
				pattern *important.txt
				monitor_type poll
				latency 100
			}
		}
	`)

	assert.Nil(t, err)
	assert.Equal(t, 1, len(app.Watch))
	assert.Equal(t, "/path", app.Watch[0].Dirs[0])
	assert.False(t, app.Watch[0].Recursive)
	assert.True(t, app.Watch[0].FollowSymlinks)
	assert.True(t, app.Watch[0].CaseSensitive)
	assert.True(t, app.Watch[0].ExtendedRegex)
	assert.Equal(t, "*important.txt", app.Watch[0].IncludeFiles)
	assert.Equal(t, "*.txt", app.Watch[0].ExcludeFiles)
	assert.Equal(t, "*important.txt", app.Watch[0].WildcardPattern)
	assert.Equal(t, "poll", app.Watch[0].MonitorType)
	assert.Equal(t, 100, app.Watch[0].Latency)
}

func parseTestConfig(config string) (*caddy.FrankenPHPApp, error) {
	app := caddy.FrankenPHPApp{}
	err := app.UnmarshalCaddyfile(caddyfile.NewTestDispenser(config))
	return &app, err
}
