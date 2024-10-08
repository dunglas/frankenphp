//go:build watcher

package caddy_test

import (
	"net/http"
	"testing"

	"github.com/caddyserver/caddy/v2/caddytest"
)

func TestWorkerWithInactiveWatcher(t *testing.T) {
	tester := caddytest.NewTester(t)
	tester.InitServer(`
		{
			skip_install_trust
			admin localhost:2999
			http_port 9080

			frankenphp {
				worker {
					file ../testdata/worker-with-watcher.php
					num 1
					watch ./**/*.php
				}
			}
		}

		localhost:9080 {
			root ../testdata
			rewrite worker-with-watcher.php
			php
		}
		`, "caddyfile")

	tester.AssertGetResponse("http://localhost:9080", http.StatusOK, "requests:1")
	tester.AssertGetResponse("http://localhost:9080", http.StatusOK, "requests:2")
}
