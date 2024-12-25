package caddy_test

import (
	"fmt"
	"io"
	"net/http"
	"path/filepath"
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

func TestRemoveWorkerThreadsViaAdminApi(t *testing.T) {
	absWorkerPath, _ := filepath.Abs("../testdata/sleep.php")
	tester := caddytest.NewTester(t)
	tester.InitServer(`
		{
			skip_install_trust
			admin localhost:2999
			http_port `+testPort+`

			frankenphp {
				num_threads 6
				max_threads 6
				worker ../testdata/sleep.php 4
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

	// make a request to the worker to make sure it's running
	tester.AssertGetResponse("http://localhost:"+testPort, http.StatusOK, "slept for 0 ms and worked for 0 iterations")

	// remove a thread
	expectedMessage := fmt.Sprintf("New thread count: 3 %s\n", absWorkerPath)
	assertAdminResponse(t, tester, "DELETE", "threads?worker", http.StatusOK, expectedMessage)

	// remove 2 threads
	expectedMessage = fmt.Sprintf("New thread count: 1 %s\n", absWorkerPath)
	assertAdminResponse(t, tester, "DELETE", "threads?worker&count=2", http.StatusOK, expectedMessage)

	// get 400 status if removing the last thread
	assertAdminResponse(t, tester, "DELETE", "threads?worker", http.StatusBadRequest, "")

	// make a request to the worker to make sure it's still running
	tester.AssertGetResponse("http://localhost:"+testPort, http.StatusOK, "slept for 0 ms and worked for 0 iterations")
}

func TestAddWorkerThreadsViaAdminApi(t *testing.T) {
	absWorkerPath, _ := filepath.Abs("../testdata/worker-with-counter.php")
	tester := caddytest.NewTester(t)
	tester.InitServer(`
		{
			skip_install_trust
			admin localhost:2999
			http_port `+testPort+`

			frankenphp {
				max_threads 10
				num_threads 3
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

	// make a request to the worker to make sure it's running
	tester.AssertGetResponse("http://localhost:"+testPort+"/", http.StatusOK, "requests:1")

	// get 400 status if the filename is wrong
	assertAdminResponse(t, tester, "PUT", "threads?worker=wrong.php", http.StatusBadRequest, "")

	// add a thread
	expectedMessage := fmt.Sprintf("New thread count: 2 %s\n", absWorkerPath)
	assertAdminResponse(t, tester, "PUT", "threads?worker=counter.php", http.StatusOK, expectedMessage)

	// add 2 threads
	expectedMessage = fmt.Sprintf("New thread count: 4 %s\n", absWorkerPath)
	assertAdminResponse(t, tester, "PUT", "threads?worker&=counter.php&count=2", http.StatusOK, expectedMessage)

	// get 400 status if adding too many threads
	assertAdminResponse(t, tester, "PUT", "threads?worker&=counter.php&count=100", http.StatusBadRequest, "")

	// make a request to the worker to make sure it's still running
	tester.AssertGetResponse("http://localhost:"+testPort+"/", http.StatusOK, "requests:2")
}

func TestShowTheCorrectThreadDebugStatus(t *testing.T) {
	tester := caddytest.NewTester(t)
	tester.InitServer(`
		{
			skip_install_trust
			admin localhost:2999
			http_port `+testPort+`

			frankenphp {
				num_threads 6
				max_threads 12
				worker ../testdata/worker-with-counter.php 2
				worker ../testdata/index.php 2
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

	// should create a 'worker-with-counter.php' thread at index 6
	assertAdminResponse(t, tester, "PUT", "threads?worker=counter.php", http.StatusOK, "")
	// should remove the 'index.php' worker thread at index 5
	assertAdminResponse(t, tester, "DELETE", "threads?worker=index.php", http.StatusOK, "")
	// should remove a regular thread at index 1
	assertAdminResponse(t, tester, "DELETE", "threads", http.StatusOK, "")

	threadInfo := getAdminResponseBody(t, tester, "GET", "threads")

	// assert that the correct threads are present in the thread info
	assert.Contains(t, threadInfo, "Thread 0")
	assert.NotContains(t, threadInfo, "Thread 1")
	assert.Contains(t, threadInfo, "Thread 2")
	assert.Contains(t, threadInfo, "Thread 3")
	assert.Contains(t, threadInfo, "Thread 4")
	assert.NotContains(t, threadInfo, "Thread 5")
	assert.Contains(t, threadInfo, "Thread 6")
	assert.NotContains(t, threadInfo, "Thread 7")
	assert.Contains(t, threadInfo, "7 additional threads can be started at runtime")
}

func TestAutoScaleWorkerThreads(t *testing.T) {
	wg := sync.WaitGroup{}
	maxTries := 100
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
	endpoint := "http://localhost:" + testPort + "/?sleep=5&work=1000"
	autoScaledThread := "Thread 2"

	// first assert that the thread is not already present
	threadInfo := getAdminResponseBody(t, tester, "GET", "threads")
	assert.NotContains(t, threadInfo, autoScaledThread)

	// try to spawn the additional threads by spamming the server
	for tries := 0; tries < maxTries; tries++ {
		wg.Add(requestsPerTry)
		for i := 0; i < requestsPerTry; i++ {
			go func() {
				tester.AssertGetResponse(endpoint, http.StatusOK, "slept for 5 ms and worked for 1000 iterations")
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

func TestAutoScaleRegularThreads(t *testing.T) {
	wg := sync.WaitGroup{}
	maxTries := 100
	requestsPerTry := 200
	tester := caddytest.NewTester(t)
	tester.InitServer(`
		{
			skip_install_trust
			admin localhost:2999
			http_port `+testPort+`

			frankenphp {
				max_threads 10
				num_threads 1
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
	endpoint := "http://localhost:" + testPort + "/sleep.php?sleep=5&work=1000"
	autoScaledThread := "Thread 1"

	// first assert that the thread is not already present
	threadInfo := getAdminResponseBody(t, tester, "GET", "threads")
	assert.NotContains(t, threadInfo, autoScaledThread)

	// try to spawn the additional threads by spamming the server
	for tries := 0; tries < maxTries; tries++ {
		wg.Add(requestsPerTry)
		for i := 0; i < requestsPerTry; i++ {
			go func() {
				tester.AssertGetResponse(endpoint, http.StatusOK, "slept for 5 ms and worked for 1000 iterations")
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
