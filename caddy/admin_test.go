package caddy_test

import (
	"encoding/json"
	"io"
	"net/http"
	"sync"
	"testing"

	"github.com/caddyserver/caddy/v2/caddytest"
	"github.com/dunglas/frankenphp"
	"github.com/stretchr/testify/assert"
)

func TestRestartWorkerViaAdminApi(t *testing.T) {
	tester := caddytest.NewTester(t)
	tester.InitServer(`
		{
			skip_install_trust
			admin localhost:2999
			http_port `+testPort+`

			frankenphp {
				worker ../testdata/worker-with-counter.php 1
			}
		}

		localhost:`+testPort+` {
			route {
				root ../testdata
				rewrite worker-with-counter.php
				php
			}
		}
		`, "caddyfile")

	tester.AssertGetResponse("http://localhost:"+testPort+"/", http.StatusOK, "requests:1")
	tester.AssertGetResponse("http://localhost:"+testPort+"/", http.StatusOK, "requests:2")

	assertAdminResponse(t, tester, "POST", "workers/restart", http.StatusOK, "workers restarted successfully\n")

	tester.AssertGetResponse("http://localhost:"+testPort+"/", http.StatusOK, "requests:1")
}

func TestShowTheCorrectThreadDebugStatus(t *testing.T) {
	tester := caddytest.NewTester(t)
	tester.InitServer(`
		{
			skip_install_trust
			admin localhost:2999
			http_port `+testPort+`

			frankenphp {
				num_threads 3
				max_threads 6
				worker ../testdata/worker-with-counter.php 1
				worker ../testdata/index.php 1
			}
		}

		localhost:`+testPort+` {
			route {
				root ../testdata
				rewrite worker-with-counter.php
				php
			}
		}
		`, "caddyfile")

	debugState := getDebugState(t, tester)

	// assert that the correct threads are present in the thread info
	assert.Equal(t, debugState.ThreadDebugStates[0].State, "ready")
	assert.Contains(t, debugState.ThreadDebugStates[1].Name, "worker-with-counter.php")
	assert.Contains(t, debugState.ThreadDebugStates[2].Name, "index.php")
	assert.Equal(t, debugState.ReservedThreadCount, 3)
	assert.Len(t, debugState.ThreadDebugStates, 3)
}

func TestAutoScaleWorkerThreads(t *testing.T) {
	wg := sync.WaitGroup{}
	maxTries := 10
	requestsPerTry := 200
	tester := caddytest.NewTester(t)
	tester.InitServer(`
		{
			skip_install_trust
			admin localhost:2999
			http_port `+testPort+`

			frankenphp {
				max_threads 10
				num_threads 2
				worker ../testdata/sleep.php 1
			}
		}

		localhost:`+testPort+` {
			route {
				root ../testdata
				rewrite sleep.php
				php
			}
		}
		`, "caddyfile")

	// spam an endpoint that simulates IO
	endpoint := "http://localhost:" + testPort + "/?sleep=2&work=1000"
	amountOfThreads := len(getDebugState(t, tester).ThreadDebugStates)

	// try to spawn the additional threads by spamming the server
	for tries := 0; tries < maxTries; tries++ {
		wg.Add(requestsPerTry)
		for i := 0; i < requestsPerTry; i++ {
			go func() {
				tester.AssertGetResponse(endpoint, http.StatusOK, "slept for 2 ms and worked for 1000 iterations")
				wg.Done()
			}()
		}
		wg.Wait()

		amountOfThreads = len(getDebugState(t, tester).ThreadDebugStates)
		if amountOfThreads > 2 {
			break
		}
	}

	// assert that there are now more threads than before
	assert.NotEqual(t, amountOfThreads, 2)
}

// Note this test requires at least 2x40MB available memory for the process
func TestAutoScaleRegularThreadsOnAutomaticThreadLimit(t *testing.T) {
	wg := sync.WaitGroup{}
	maxTries := 10
	requestsPerTry := 200
	tester := caddytest.NewTester(t)
	tester.InitServer(`
		{
			skip_install_trust
			admin localhost:2999
			http_port `+testPort+`

			frankenphp {
				max_threads auto
				num_threads 1
				php_ini memory_limit 40M # a reasonable limit for the test
			}
		}

		localhost:`+testPort+` {
			route {
				root ../testdata
				php
			}
		}
		`, "caddyfile")

	// spam an endpoint that simulates IO
	endpoint := "http://localhost:" + testPort + "/sleep.php?sleep=2&work=1000"
	amountOfThreads := len(getDebugState(t, tester).ThreadDebugStates)

	// try to spawn the additional threads by spamming the server
	for tries := 0; tries < maxTries; tries++ {
		wg.Add(requestsPerTry)
		for i := 0; i < requestsPerTry; i++ {
			go func() {
				tester.AssertGetResponse(endpoint, http.StatusOK, "slept for 2 ms and worked for 1000 iterations")
				wg.Done()
			}()
		}
		wg.Wait()

		amountOfThreads = len(getDebugState(t, tester).ThreadDebugStates)
		if amountOfThreads > 1 {
			break
		}
	}

	// assert that there are now more threads present
	assert.NotEqual(t, amountOfThreads, 1)
}

func assertAdminResponse(t *testing.T, tester *caddytest.Tester, method string, path string, expectedStatus int, expectedBody string) {
	adminUrl := "http://localhost:2999/frankenphp/"
	r, err := http.NewRequest(method, adminUrl+path, nil)
	assert.NoError(t, err)
	if expectedBody == "" {
		_ = tester.AssertResponseCode(r, expectedStatus)
		return
	}
	_, _ = tester.AssertResponse(r, expectedStatus, expectedBody)
}

func getAdminResponseBody(t *testing.T, tester *caddytest.Tester, method string, path string) string {
	adminUrl := "http://localhost:2999/frankenphp/"
	r, err := http.NewRequest(method, adminUrl+path, nil)
	assert.NoError(t, err)
	resp := tester.AssertResponseCode(r, http.StatusOK)
	defer resp.Body.Close()
	bytes, err := io.ReadAll(resp.Body)
	assert.NoError(t, err)

	return string(bytes)
}

func getDebugState(t *testing.T, tester *caddytest.Tester) frankenphp.FrankenPHPDebugState {
	threadStates := getAdminResponseBody(t, tester, "GET", "threads")

	var debugStates frankenphp.FrankenPHPDebugState
	err := json.Unmarshal([]byte(threadStates), &debugStates)
	assert.NoError(t, err)

	return debugStates
}
