package caddy_test

import (
	"github.com/caddyserver/caddy/v2/caddytest"
	"net/http"
	"testing"
	"fmt"
	"path/filepath"
)

func TestRestartingWorkerViaAdminApi(t *testing.T) {
	tester := caddytest.NewTester(t)
	tester.InitServer(`
		{
			skip_install_trust
			admin localhost:2999
			http_port `+testPort+`
			https_port 9443

			frankenphp {
				worker ../testdata/worker-with-watcher.php 1
			}
		}

		localhost:`+testPort+` {
			route {
				root ../testdata
				rewrite worker-with-watcher.php
				php
			}
		}
		`, "caddyfile")

	tester.AssertGetResponse("http://localhost:"+testPort+"/", http.StatusOK, "requests:1")
	tester.AssertGetResponse("http://localhost:"+testPort+"/", http.StatusOK, "requests:2")

	tester.AssertGetResponse("http://localhost:2999/frankenphp/workers/restart", http.StatusOK, "workers restarted successfully\n")

	tester.AssertGetResponse("http://localhost:"+testPort+"/", http.StatusOK, "requests:1")
}

func TestRemovingAndAddingAThreadViaAdminApi(t *testing.T) {
	absWorkerPath, _ := filepath.Abs("../testdata/worker-with-watcher.php")
	tester := caddytest.NewTester(t)
	tester.InitServer(`
		{
			skip_install_trust
			admin localhost:2999
			http_port `+testPort+`
			https_port 9443

			frankenphp {
				worker ../testdata/worker-with-watcher.php 2
			}
		}

		localhost:`+testPort+` {
			route {
				root ../testdata
				rewrite worker-with-watcher.php
				php
			}
		}
		`, "caddyfile")

	// make a request to the worker to make sure it's running
	tester.AssertGetResponse("http://localhost:"+testPort+"/", http.StatusOK, "requests:1")

	// remove a thread
	expectedMessage := fmt.Sprintf("New thread count: 1 %s\n",absWorkerPath)
	tester.AssertGetResponse("http://localhost:2999/frankenphp/workers/remove", http.StatusOK, expectedMessage)

	// TODO: try removing the last thread
	//tester.AssertResponseCode("http://localhost:2999/frankenphp/workers/remove", http.StatusInternalServerError)

	// make a request to the worker to make sure it's still running
	tester.AssertGetResponse("http://localhost:"+testPort+"/", http.StatusOK, "requests:2")

	// add a thread
	expectedMessage = fmt.Sprintf("New thread count: 2 %s\n",absWorkerPath)
    tester.AssertGetResponse("http://localhost:2999/frankenphp/workers/add", http.StatusOK, expectedMessage)

	// make a request to the worker to make sure it's still running
	tester.AssertGetResponse("http://localhost:"+testPort+"/", http.StatusOK, "requests:3")
}