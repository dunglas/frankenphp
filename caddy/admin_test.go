package caddy_test

import (
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/caddyserver/caddy/v2/caddytest"
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

	threadInfo := getAdminResponseBody(t, tester, "GET", "threads")

	// assert that the correct threads are present in the thread info
	assert.Contains(t, threadInfo, "Thread 0")
	assert.Contains(t, threadInfo, "Thread 1")
	assert.Contains(t, threadInfo, "Thread 2")
	assert.NotContains(t, threadInfo, "Thread 3")
	assert.Contains(t, threadInfo, "3 additional threads can be started at runtime")
	assert.Contains(t, threadInfo, "worker-with-counter.php")
	assert.Contains(t, threadInfo, "index.php")
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
	autoScaledThread := "Thread 2"

	// first assert that the thread is not already present
	threadInfo := getAdminResponseBody(t, tester, "GET", "threads")
	assert.NotContains(t, threadInfo, autoScaledThread)

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
		threadInfo = getAdminResponseBody(t, tester, "GET", "threads")
		if strings.Contains(threadInfo, autoScaledThread) {
			break
		}
	}

	// assert that the autoscaled thread is present in the threadInfo
	assert.Contains(t, threadInfo, autoScaledThread)
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
	autoScaledThread := "Thread 1"

	// first assert that the thread is not already present
	threadInfo := getAdminResponseBody(t, tester, "GET", "threads")
	assert.NotContains(t, threadInfo, autoScaledThread)

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
		threadInfo = getAdminResponseBody(t, tester, "GET", "threads")
		if strings.Contains(threadInfo, autoScaledThread) {
			break
		}
	}

	// assert that the autoscaled thread is present in the threadInfo
	assert.Contains(t, threadInfo, autoScaledThread)
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
