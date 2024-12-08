package caddy_test

import (
	"fmt"
	"github.com/caddyserver/caddy/v2/caddytest"
	"net/http"
	"path/filepath"
	"testing"
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

	assertAdminResponse(tester, "POST", "restart", http.StatusOK, "workers restarted successfully\n")

	tester.AssertGetResponse("http://localhost:"+testPort+"/", http.StatusOK, "requests:1")
}

func TestRemoveThreadsViaAdminApi(t *testing.T) {
	absWorkerPath, _ := filepath.Abs("../testdata/worker-with-watcher.php")
	tester := caddytest.NewTester(t)
	tester.InitServer(`
		{
			skip_install_trust
			admin localhost:2999
			http_port `+testPort+`
			https_port 9443

			frankenphp {
				worker ../testdata/worker-with-watcher.php 4
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
	expectedMessage := fmt.Sprintf("New thread count: 3 %s\n", absWorkerPath)
	assertAdminResponse(tester, "POST", "remove", http.StatusOK, expectedMessage)

	// remove 2 threads
	expectedMessage = fmt.Sprintf("New thread count: 1 %s\n", absWorkerPath)
	assertAdminResponse(tester, "POST", "remove?count=2", http.StatusOK, expectedMessage)

	// get 400 status if removing the last thread
	assertAdminResponse(tester, "POST", "remove", http.StatusBadRequest, "")

	// make a request to the worker to make sure it's still running
	tester.AssertGetResponse("http://localhost:"+testPort+"/", http.StatusOK, "requests:2")
}

func TestAddThreadsViaAdminApi(t *testing.T) {
	absWorkerPath, _ := filepath.Abs("../testdata/worker-with-watcher.php")
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

	// make a request to the worker to make sure it's running
	tester.AssertGetResponse("http://localhost:"+testPort+"/", http.StatusOK, "requests:1")

	// get 400 status if the filename is wrong
	assertAdminResponse(tester, "POST", "add?file=wrong.php", http.StatusBadRequest, "")

	// add a thread
	expectedMessage := fmt.Sprintf("New thread count: 2 %s\n", absWorkerPath)
	assertAdminResponse(tester, "POST", "add", http.StatusOK, expectedMessage)

	// add 2 threads
	expectedMessage = fmt.Sprintf("New thread count: 4 %s\n", absWorkerPath)
	assertAdminResponse(tester, "POST", "add?count=2", http.StatusOK, expectedMessage)

	// get 400 status if adding too many threads
	assertAdminResponse(tester, "POST", "add?count=100", http.StatusBadRequest, "")

	// make a request to the worker to make sure it's still running
	tester.AssertGetResponse("http://localhost:"+testPort+"/", http.StatusOK, "requests:2")
}

func assertAdminResponse(tester *caddytest.Tester, method string, path string, expectedStatus int, expectedBody string) {
	adminUrl := "http://localhost:2999/frankenphp/workers/"
	r, err := http.NewRequest(method, adminUrl+path, nil)
	if err != nil {
		panic(err)
	}
	if expectedBody == "" {
		tester.AssertResponseCode(r, expectedStatus)
	} else {
		tester.AssertResponse(r, expectedStatus, expectedBody)
	}
}
