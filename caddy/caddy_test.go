package caddy_test

import (
	"bytes"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/dunglas/frankenphp"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/require"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddytest"
)

var testPort = "9080"

func TestPHP(t *testing.T) {
	var wg sync.WaitGroup
	tester := caddytest.NewTester(t)
	tester.InitServer(`
		{
			skip_install_trust
			admin localhost:2999
			http_port `+testPort+`
			https_port 9443

			frankenphp
		}

		localhost:`+testPort+` {
			route {
				php {
					root ../testdata
				}
			}
		}
		`, "caddyfile")

	for i := 0; i < 100; i++ {
		wg.Add(1)

		go func(i int) {
			tester.AssertGetResponse(fmt.Sprintf("http://localhost:"+testPort+"/index.php?i=%d", i), http.StatusOK, fmt.Sprintf("I am by birth a Genevese (%d)", i))
			wg.Done()
		}(i)
	}
	wg.Wait()
}

func TestLargeRequest(t *testing.T) {
	tester := caddytest.NewTester(t)
	tester.InitServer(`
		{
			skip_install_trust
			admin localhost:2999
			http_port `+testPort+`
			https_port 9443

			frankenphp
		}

		localhost:`+testPort+` {
			route {
				php {
					root ../testdata
				}
			}
		}
		`, "caddyfile")

	tester.AssertPostResponseBody(
		"http://localhost:"+testPort+"/large-request.php",
		[]string{},
		bytes.NewBufferString(strings.Repeat("f", 1_048_576)),
		http.StatusOK,
		"Request body size: 1048576 (unknown)",
	)
}

func TestWorker(t *testing.T) {
	var wg sync.WaitGroup
	tester := caddytest.NewTester(t)
	tester.InitServer(`
		{
			skip_install_trust
			admin localhost:2999
			http_port `+testPort+`
			https_port 9443

			frankenphp {
				worker ../testdata/index.php 2
			}
		}

		localhost:`+testPort+` {
			route {
				php {
					root ../testdata
				}
			}
		}
		`, "caddyfile")

	for i := 0; i < 100; i++ {
		wg.Add(1)

		go func(i int) {
			tester.AssertGetResponse(fmt.Sprintf("http://localhost:"+testPort+"/index.php?i=%d", i), http.StatusOK, fmt.Sprintf("I am by birth a Genevese (%d)", i))
			wg.Done()
		}(i)
	}
	wg.Wait()
}

func TestEnv(t *testing.T) {
	tester := caddytest.NewTester(t)
	tester.InitServer(`
		{
			skip_install_trust
			admin localhost:2999
			http_port `+testPort+`
			https_port 9443

			frankenphp {
				worker {
					file ../testdata/worker-env.php
					num 1
					env FOO bar
				}
			}
		}

		localhost:`+testPort+` {
			route {
				php {
					root ../testdata
					env FOO baz
				}
			}
		}
		`, "caddyfile")

	tester.AssertGetResponse("http://localhost:"+testPort+"/worker-env.php", http.StatusOK, "bazbar")
}

func TestJsonEnv(t *testing.T) {
	tester := caddytest.NewTester(t)
	tester.InitServer(`
		{
		"admin": {
			"listen": "localhost:2999"
		},
		"apps": {
			"frankenphp": {
			"workers": [
				{
				"env": {
					"FOO": "bar"
				},
				"file_name": "../testdata/worker-env.php",
				"num": 1
				}
			]
			},
			"http": {
			"http_port": `+testPort+`,
			"https_port": 9443,
			"servers": {
				"srv0": {
				"listen": [
					":`+testPort+`"
				],
				"routes": [
					{
					"handle": [
						{
						"handler": "subroute",
						"routes": [
							{
							"handle": [
								{
								"handler": "subroute",
								"routes": [
									{
									"handle": [
										{
										"env": {
											"FOO": "baz"
										},
										"handler": "php",
										"root": "../testdata"
										}
									]
									}
								]
								}
							]
							}
						]
						}
					],
					"match": [
						{
						"host": [
							"localhost"
						]
						}
					],
					"terminal": true
					}
				]
				}
			}
			},
			"pki": {
			"certificate_authorities": {
				"local": {
				"install_trust": false
				}
			}
			}
		}
		}
		`, "json")

	tester.AssertGetResponse("http://localhost:"+testPort+"/worker-env.php", http.StatusOK, "bazbar")
}

func TestCustomCaddyVariablesInEnv(t *testing.T) {
	tester := caddytest.NewTester(t)
	tester.InitServer(`
		{
			skip_install_trust
			admin localhost:2999
			http_port `+testPort+`
			https_port 9443

			frankenphp {
				worker {
					file ../testdata/worker-env.php
					num 1
					env FOO world
				}
			}
		}

		localhost:`+testPort+` {
			route {
				map 1 {my_customvar} {
					default "hello "
				}
				php {
					root ../testdata
					env FOO {my_customvar}
				}
			}
		}
		`, "caddyfile")

	tester.AssertGetResponse("http://localhost:"+testPort+"/worker-env.php", http.StatusOK, "hello world")
}

func TestPHPServerDirective(t *testing.T) {
	tester := caddytest.NewTester(t)
	tester.InitServer(`
		{
			skip_install_trust
			admin localhost:2999
			http_port `+testPort+`
			https_port 9443

			frankenphp
		}

		localhost:`+testPort+` {
			root * ../testdata
			php_server
		}
		`, "caddyfile")

	tester.AssertGetResponse("http://localhost:"+testPort, http.StatusOK, "I am by birth a Genevese (i not set)")
	tester.AssertGetResponse("http://localhost:"+testPort+"/hello.txt", http.StatusOK, "Hello")
	tester.AssertGetResponse("http://localhost:"+testPort+"/not-found.txt", http.StatusOK, "I am by birth a Genevese (i not set)")
}

func TestPHPServerDirectiveDisableFileServer(t *testing.T) {
	tester := caddytest.NewTester(t)
	tester.InitServer(`
		{
			skip_install_trust
			admin localhost:2999
			http_port `+testPort+`
			https_port 9443

			frankenphp
			order php_server before respond
		}

		localhost:`+testPort+` {
			root * ../testdata
			php_server {
				file_server off
			}
			respond "Not found" 404
		}
		`, "caddyfile")

	tester.AssertGetResponse("http://localhost:"+testPort, http.StatusOK, "I am by birth a Genevese (i not set)")
	tester.AssertGetResponse("http://localhost:"+testPort+"/not-found.txt", http.StatusOK, "I am by birth a Genevese (i not set)")
}

func TestMetrics(t *testing.T) {
	var wg sync.WaitGroup
	tester := caddytest.NewTester(t)
	tester.InitServer(`
	{
		skip_install_trust
		admin localhost:2999
		http_port `+testPort+`
		https_port 9443

		frankenphp
	}

	localhost:`+testPort+` {
		route {
			mercure {
				transport local
				anonymous
				publisher_jwt !ChangeMe!
			}

			php {
				root ../testdata
			}
		}
	}

	example.com:`+testPort+` {
		route {
			mercure {
				transport local
				anonymous
				publisher_jwt !ChangeMe!
			}

			php {
				root ../testdata
			}
		}
	}
	`, "caddyfile")

	// Make some requests
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			tester.AssertGetResponse(fmt.Sprintf("http://localhost:"+testPort+"/index.php?i=%d", i), http.StatusOK, fmt.Sprintf("I am by birth a Genevese (%d)", i))
			wg.Done()
		}(i)
	}
	wg.Wait()

	// Fetch metrics
	resp, err := http.Get("http://localhost:2999/metrics")
	if err != nil {
		t.Fatalf("failed to fetch metrics: %v", err)
	}
	defer resp.Body.Close()

	// Read and parse metrics
	metrics := new(bytes.Buffer)
	_, err = metrics.ReadFrom(resp.Body)
	if err != nil {
		t.Fatalf("failed to read metrics: %v", err)
	}

	cpus := fmt.Sprintf("%d", frankenphp.MaxThreads)

	// Check metrics
	expectedMetrics := `
	# HELP frankenphp_total_threads Total number of PHP threads
	# TYPE frankenphp_total_threads counter
	frankenphp_total_threads ` + cpus + `

	# HELP frankenphp_busy_threads Number of busy PHP threads
	# TYPE frankenphp_busy_threads gauge
	frankenphp_busy_threads 0
	`

	ctx := caddy.ActiveContext()

	require.NoError(t, testutil.GatherAndCompare(ctx.GetMetricsRegistry(), strings.NewReader(expectedMetrics), "frankenphp_total_threads", "frankenphp_busy_threads"))
}

func TestWorkerMetrics(t *testing.T) {
	var wg sync.WaitGroup
	tester := caddytest.NewTester(t)
	tester.InitServer(`
	{
		skip_install_trust
		admin localhost:2999
		http_port `+testPort+`
		https_port 9443

		frankenphp {
			worker ../testdata/index.php 2
		}
	}

	localhost:`+testPort+` {
		route {
			php {
				root ../testdata
			}
		}
	}

	example.com:`+testPort+` {
		route {
			php {
				root ../testdata
			}
		}
	}
	`, "caddyfile")

	// Make some requests
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			tester.AssertGetResponse(fmt.Sprintf("http://localhost:"+testPort+"/index.php?i=%d", i), http.StatusOK, fmt.Sprintf("I am by birth a Genevese (%d)", i))
			wg.Done()
		}(i)
	}
	wg.Wait()

	// Fetch metrics
	resp, err := http.Get("http://localhost:2999/metrics")
	if err != nil {
		t.Fatalf("failed to fetch metrics: %v", err)
	}
	defer resp.Body.Close()

	// Read and parse metrics
	metrics := new(bytes.Buffer)
	_, err = metrics.ReadFrom(resp.Body)
	if err != nil {
		t.Fatalf("failed to read metrics: %v", err)
	}

	cpus := fmt.Sprintf("%d", frankenphp.MaxThreads)

	// Check metrics
	expectedMetrics := `
	# HELP frankenphp_total_threads Total number of PHP threads
	# TYPE frankenphp_total_threads counter
	frankenphp_total_threads ` + cpus + `

	# HELP frankenphp_busy_threads Number of busy PHP threads
	# TYPE frankenphp_busy_threads gauge
	frankenphp_busy_threads 2

	# HELP frankenphp_testdata_index_php_busy_workers Number of busy PHP workers for this worker
	# TYPE frankenphp_testdata_index_php_busy_workers gauge
	frankenphp_testdata_index_php_busy_workers 0

	# HELP frankenphp_testdata_index_php_total_workers Total number of PHP workers for this worker
	# TYPE frankenphp_testdata_index_php_total_workers gauge
	frankenphp_testdata_index_php_total_workers 2

	# HELP frankenphp_testdata_index_php_worker_request_count
	# TYPE frankenphp_testdata_index_php_worker_request_count counter
	frankenphp_testdata_index_php_worker_request_count 10

	# HELP frankenphp_testdata_index_php_ready_workers Running workers that have successfully called frankenphp_handle_request at least once
	# TYPE frankenphp_testdata_index_php_ready_workers gauge
	frankenphp_testdata_index_php_ready_workers 2

	# HELP frankenphp_testdata_index_php_worker_crashes Number of PHP worker crashes for this worker
	# TYPE frankenphp_testdata_index_php_worker_crashes counter
	frankenphp_testdata_index_php_worker_crashes 0

	# HELP frankenphp_testdata_index_php_worker_restarts Number of PHP worker restarts for this worker
	# TYPE frankenphp_testdata_index_php_worker_restarts counter
	frankenphp_testdata_index_php_worker_restarts 0
	`

	ctx := caddy.ActiveContext()
	require.NoError(t,
		testutil.GatherAndCompare(
			ctx.GetMetricsRegistry(),
			strings.NewReader(expectedMetrics),
			"frankenphp_total_threads",
			"frankenphp_busy_threads",
			"frankenphp_testdata_index_php_busy_workers",
			"frankenphp_testdata_index_php_total_workers",
			"frankenphp_testdata_index_php_worker_request_count",
			"frankenphp_testdata_index_php_worker_crashes",
			"frankenphp_testdata_index_php_worker_restarts",
			"frankenphp_testdata_index_php_ready_workers",
		))
}

func TestAutoWorkerConfig(t *testing.T) {
	var wg sync.WaitGroup
	tester := caddytest.NewTester(t)
	tester.InitServer(`
	{
		skip_install_trust
		admin localhost:2999
		http_port `+testPort+`
		https_port 9443

		frankenphp {
			worker ../testdata/index.php
		}
	}

	localhost:`+testPort+` {
		route {
			php {
				root ../testdata
			}
		}
	}
	`, "caddyfile")

	// Make some requests
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			tester.AssertGetResponse(fmt.Sprintf("http://localhost:"+testPort+"/index.php?i=%d", i), http.StatusOK, fmt.Sprintf("I am by birth a Genevese (%d)", i))
			wg.Done()
		}(i)
	}
	wg.Wait()

	// Fetch metrics
	resp, err := http.Get("http://localhost:2999/metrics")
	if err != nil {
		t.Fatalf("failed to fetch metrics: %v", err)
	}
	defer resp.Body.Close()

	// Read and parse metrics
	metrics := new(bytes.Buffer)
	_, err = metrics.ReadFrom(resp.Body)
	if err != nil {
		t.Fatalf("failed to read metrics: %v", err)
	}

	cpus := fmt.Sprintf("%d", frankenphp.MaxThreads)
	workers := fmt.Sprintf("%d", frankenphp.MaxThreads-1)

	// Check metrics
	expectedMetrics := `
	# HELP frankenphp_total_threads Total number of PHP threads
	# TYPE frankenphp_total_threads counter
	frankenphp_total_threads ` + cpus + `

	# HELP frankenphp_busy_threads Number of busy PHP threads
	# TYPE frankenphp_busy_threads gauge
	frankenphp_busy_threads ` + workers + `

	# HELP frankenphp_testdata_index_php_busy_workers Number of busy PHP workers for this worker
	# TYPE frankenphp_testdata_index_php_busy_workers gauge
	frankenphp_testdata_index_php_busy_workers 0

	# HELP frankenphp_testdata_index_php_total_workers Total number of PHP workers for this worker
	# TYPE frankenphp_testdata_index_php_total_workers gauge
	frankenphp_testdata_index_php_total_workers ` + workers + `

	# HELP frankenphp_testdata_index_php_worker_request_count
	# TYPE frankenphp_testdata_index_php_worker_request_count counter
	frankenphp_testdata_index_php_worker_request_count 10

	# HELP frankenphp_testdata_index_php_ready_workers Running workers that have successfully called frankenphp_handle_request at least once
	# TYPE frankenphp_testdata_index_php_ready_workers gauge
	frankenphp_testdata_index_php_ready_workers ` + workers + `

	# HELP frankenphp_testdata_index_php_worker_crashes Number of PHP worker crashes for this worker
	# TYPE frankenphp_testdata_index_php_worker_crashes counter
	frankenphp_testdata_index_php_worker_crashes 0

	# HELP frankenphp_testdata_index_php_worker_restarts Number of PHP worker restarts for this worker
	# TYPE frankenphp_testdata_index_php_worker_restarts counter
	frankenphp_testdata_index_php_worker_restarts 0
	`

	ctx := caddy.ActiveContext()
	require.NoError(t,
		testutil.GatherAndCompare(
			ctx.GetMetricsRegistry(),
			strings.NewReader(expectedMetrics),
			"frankenphp_total_threads",
			"frankenphp_busy_threads",
			"frankenphp_testdata_index_php_busy_workers",
			"frankenphp_testdata_index_php_total_workers",
			"frankenphp_testdata_index_php_worker_request_count",
			"frankenphp_testdata_index_php_worker_crashes",
			"frankenphp_testdata_index_php_worker_restarts",
			"frankenphp_testdata_index_php_ready_workers",
		))
}

func TestAllDefinedServerVars(t *testing.T) {
	documentRoot, _ := filepath.Abs("../testdata/")
	expectedBodyFile, _ := os.ReadFile("../testdata/server-all-vars-ordered.txt")
	expectedBody := string(expectedBodyFile)
	expectedBody = strings.ReplaceAll(expectedBody, "{documentRoot}", documentRoot)
	expectedBody = strings.ReplaceAll(expectedBody, "\r\n", "\n")
	expectedBody = strings.ReplaceAll(expectedBody, "{testPort}", testPort)
	tester := caddytest.NewTester(t)
	tester.InitServer(`
		{
			skip_install_trust
			admin localhost:2999
			http_port `+testPort+`
			frankenphp
		}
		localhost:`+testPort+` {
			route {
			    root ../testdata
			    # rewrite to test that the original path is passed as $REQUEST_URI
			    rewrite /server-all-vars-ordered.php/path
				php
			}
		}
		`, "caddyfile")
	tester.AssertPostResponseBody(
		"http://user@localhost:"+testPort+"/original-path?specialChars=%3E\\x00%00</>",
		[]string{
			"Content-Type: application/x-www-form-urlencoded",
			"Content-Length: 14", // maliciously set to 14
			"Special-Chars: <%00>",
			"Host: Malicious Host",
			"X-Empty-Header:",
		},
		bytes.NewBufferString("foo=bar"),
		http.StatusOK,
		expectedBody,
	)
}

func TestPHPIniConfiguration(t *testing.T) {
	tester := caddytest.NewTester(t)
	tester.InitServer(`
		{
			skip_install_trust
			admin localhost:2999
			http_port `+testPort+`

			frankenphp {
				num_threads 2
				worker ../testdata/ini.php 1
				php_ini upload_max_filesize 100M
				php_ini memory_limit 10000000
			}
		}

		localhost:`+testPort+` {
			route {
				root ../testdata
				php
			}
		}
		`, "caddyfile")

	testSingleIniConfiguration(tester, "upload_max_filesize", "100M")
	testSingleIniConfiguration(tester, "memory_limit", "10000000")
}

func TestPHPIniBlockConfiguration(t *testing.T) {
	tester := caddytest.NewTester(t)
	tester.InitServer(`
		{
			skip_install_trust
			admin localhost:2999
			http_port `+testPort+`

			frankenphp {
				num_threads 1
				php_ini {
					upload_max_filesize 100M
					memory_limit 20000000
				}
			}
		}

		localhost:`+testPort+` {
			route {
				root ../testdata
				php
			}
		}
		`, "caddyfile")

	testSingleIniConfiguration(tester, "upload_max_filesize", "100M")
	testSingleIniConfiguration(tester, "memory_limit", "20000000")
}

func testSingleIniConfiguration(tester *caddytest.Tester, key string, value string) {
	// test twice to ensure the ini setting is not lost
	for i := 0; i < 2; i++ {
		tester.AssertGetResponse(
			"http://localhost:"+testPort+"/ini.php?key="+key,
			http.StatusOK,
			key+":"+value,
		)
	}
}

func TestOsEnv(t *testing.T) {
	os.Setenv("ENV1", "value1")
	os.Setenv("ENV2", "value2")
	tester := caddytest.NewTester(t)
	tester.InitServer(`
		{
			skip_install_trust
			admin localhost:2999
			http_port `+testPort+`

			frankenphp {
				num_threads 2
				php_ini variables_order "EGPCS"
				worker ../testdata/env/env.php 1
			}
		}

		localhost:`+testPort+` {
			route {
				root ../testdata
				php
			}
		}
		`, "caddyfile")

	tester.AssertGetResponse(
		"http://localhost:"+testPort+"/env/env.php?keys[]=ENV1&keys[]=ENV2",
		http.StatusOK,
		"ENV1=value1,ENV2=value2",
	)
}

func TestMaxWaitTime(t *testing.T) {
	tester := caddytest.NewTester(t)
	tester.InitServer(`
		{
			skip_install_trust
			admin localhost:2999
			http_port `+testPort+`

			frankenphp {
				num_threads 1
				max_wait_time 1ns
			}
		}

		localhost:`+testPort+` {
			route {
				root ../testdata
				php
			}
		}
		`, "caddyfile")

	// send 10 requests simultaneously, at least one request should be stalled longer than 1ns
	// since we only have 1 thread, this will cause a 504 Gateway Timeout
	wg := sync.WaitGroup{}
	success := atomic.Bool{}
	wg.Add(10)
	for i := 0; i < 10; i++ {
		go func() {
			statusCode := getStatusCode("http://localhost:"+testPort+"/sleep.php?sleep=10", t)
			if statusCode == http.StatusGatewayTimeout {
				success.Store(true)
			}
			wg.Done()
		}()
	}
	wg.Wait()

	require.True(t, success.Load(), "At least one request should have failed with a 504 Gateway Timeout status")
}

func getStatusCode(url string, t *testing.T) int {
	req, err := http.NewRequest("GET", url, nil)
	require.NoError(t, err)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	return resp.StatusCode
}
