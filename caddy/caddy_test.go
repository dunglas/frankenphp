package caddy_test

import (
	"bytes"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/caddyserver/caddy/v2/caddytest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPHP(t *testing.T) {
	var wg sync.WaitGroup
	tester := caddytest.NewTester(t)
	tester.InitServer(`
		{
			skip_install_trust
			admin localhost:2999
			http_port 9080
			https_port 9443

			frankenphp
		}

		localhost:9080 {
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
			defer wg.Done()

			tester.AssertGetResponse(fmt.Sprintf("http://localhost:9080/index.php?i=%d", i), http.StatusOK, fmt.Sprintf("I am by birth a Genevese (%d)", i))
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
			http_port 9080
			https_port 9443

			frankenphp
		}

		localhost:9080 {
			route {
				php {
					root ../testdata
				}
			}
		}
		`, "caddyfile")

	tester.AssertPostResponseBody(
		"http://localhost:9080/large-request.php",
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
			http_port 9080
			https_port 9443

			frankenphp {
				worker ../testdata/index.php 2
			}
		}

		localhost:9080 {
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
			defer wg.Done()

			tester.AssertGetResponse(fmt.Sprintf("http://localhost:9080/index.php?i=%d", i), http.StatusOK, fmt.Sprintf("I am by birth a Genevese (%d)", i))
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
			http_port 9080
			https_port 9443

			frankenphp {
				worker {
					file ../testdata/env.php
					num 1
					env FOO bar
				}
			}
		}

		localhost:9080 {
			route {
				php {
					root ../testdata
					env FOO baz
				}
			}
		}
		`, "caddyfile")

	tester.AssertGetResponse("http://localhost:9080/env.php", http.StatusOK, "bazbar")
}

func TestPHPServerDirective(t *testing.T) {
	tester := caddytest.NewTester(t)
	tester.InitServer(`
		{
			skip_install_trust
			admin localhost:2999
			http_port 9080
			https_port 9443

			frankenphp
			order php_server before reverse_proxy
		}

		localhost:9080 {
			root * ../testdata
			php_server
		}
		`, "caddyfile")

	tester.AssertGetResponse("http://localhost:9080", http.StatusOK, "I am by birth a Genevese (i not set)")
	tester.AssertGetResponse("http://localhost:9080/hello.txt", http.StatusOK, "Hello")
	tester.AssertGetResponse("http://localhost:9080/not-found.txt", http.StatusOK, "I am by birth a Genevese (i not set)")
}

func TestPHPServerDirectiveDisableFileServer(t *testing.T) {
	tester := caddytest.NewTester(t)
	tester.InitServer(`
		{
			skip_install_trust
			admin localhost:2999
			http_port 9080
			https_port 9443

			frankenphp
			order php_server before respond
		}

		localhost:9080 {
			root * ../testdata
			php_server {
				file_server off
			}
			respond "Not found" 404
		}
		`, "caddyfile")

	tester.AssertGetResponse("http://localhost:9080", http.StatusOK, "I am by birth a Genevese (i not set)")
	tester.AssertGetResponse("http://localhost:9080/hello.txt", http.StatusNotFound, "Not found")
}

// TestReload sends many concurrent reload requests, as done by Laravel Octane.
// Better run this test with -race.
func TestReload(t *testing.T) {
	tester := caddytest.NewTester(t)
	tester.InitServer(`
	{
		skip_install_trust
		admin localhost:2999
		http_port 9080
		https_port 9443

		frankenphp {
			worker ../testdata/index.php
		}
		order php_server before respond
	}

	localhost:9080 {
		root * ../testdata
		php_server
	}
	`, "caddyfile")

	const configURL = "http://localhost:2999/config/apps/frankenphp"

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()

			resp1, err := tester.Client.Get(configURL)
			require.NoError(t, err)

			r, err := http.NewRequest("PATCH", configURL, resp1.Body)
			require.NoError(t, err)
			r.Header.Add("Content-Type", "application/json")
			r.Header.Add("Cache-Control", "must-revalidate")

			resp, err := tester.Client.Do(r)
			require.NoError(t, err)
			assert.Equal(t, http.StatusOK, resp.StatusCode)
		}()
	}
	wg.Wait()

	tester.AssertGetResponse("http://localhost:9080", http.StatusOK, "I am by birth a Genevese (i not set)")
}
