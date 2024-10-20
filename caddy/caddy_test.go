package caddy_test

import (
	"bytes"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/dunglas/frankenphp"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/require"

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
	tester.AssertGetResponse("http://localhost:"+testPort+"/hello.txt", http.StatusNotFound, "Not found")
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

	require.NoError(t, testutil.GatherAndCompare(prometheus.DefaultGatherer, strings.NewReader(expectedMetrics), "frankenphp_total_threads", "frankenphp_busy_threads"))
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

	require.NoError(t,
		testutil.GatherAndCompare(
			prometheus.DefaultGatherer,
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

	require.NoError(t,
		testutil.GatherAndCompare(
			prometheus.DefaultGatherer,
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

func TestAllServerVarsWithInputFilterInFringeMode(t *testing.T) {
	expectedBody, _ := os.ReadFile("../testdata/server-filter-var.txt")
	tester := caddytest.NewTester(t)
	tester.InitServer(`
		{
			skip_install_trust
			admin localhost:2999
			http_port `+testPort+`
			frankenphp {
				fringe_mode true
			}
		}
		localhost:`+testPort+` {
			route {
			    root ../testdata
				php
			}
		}
		`, "caddyfile")

	tester.AssertPostResponseBody(
		"http://user@localhost:"+testPort+"/server-filter-var.php/path?withFilterVar=true&specialChars=%3C\\x00</>",
		[]string{
			"Content-Type: application/x-www-form-urlencoded",
			"Content-Length: 14", // maliciously set to 14
			"Special-Chars: <\\x00>",
			"Host: Malicous Host",
		},
		bytes.NewBufferString("foo=bar"),
		http.StatusOK,
		string(expectedBody),
	)
}

func TestAllServerVarsWithoutInputFilter(t *testing.T) {
	expectedBody, _ := os.ReadFile("../testdata/server-filter-var.txt")
	expectedBody = bytes.ReplaceAll(expectedBody, []byte("withFilterVar=true"), []byte("withFilterVar=false"))
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
				php
			}
		}
		`, "caddyfile")
	tester.AssertPostResponseBody(
		"http://user@localhost:"+testPort+"/server-filter-var.php/path?withFilterVar=false&specialChars=%3C%3E\\x00</>",
		[]string{
			"Content-Type: application/x-www-form-urlencoded",
			"Content-Length: 14", // maliciously set to 14
			"Special-Chars: <\\x00>",
			"Host: Malicous Host",
		},
		bytes.NewBufferString("foo=bar"),
		http.StatusOK,
		string(expectedBody),
	)
}
