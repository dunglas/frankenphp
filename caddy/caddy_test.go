package caddy_test

import (
	"bytes"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/caddyserver/caddy/v2/caddytest"
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
			tester.AssertGetResponse(fmt.Sprintf("http://localhost:9080/index.php?i=%d", i), http.StatusOK, fmt.Sprintf("I am by birth a Genevese (%d)", i))
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
			tester.AssertGetResponse(fmt.Sprintf("http://localhost:9080/index.php?i=%d", i), http.StatusOK, fmt.Sprintf("I am by birth a Genevese (%d)", i))
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

func TestPhpServerXAccelRedirect(t *testing.T) {
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
				@accel header X-Accel-Redirect *
				handle_response @accel {
					root * ../testdata
				    rewrite {rp.header.X-Accel-Redirect}
				    file_server
				}
			}
			respond "Not found" 404
		}
		`, "caddyfile")

	tester.AssertGetResponse("http://localhost:9080", http.StatusOK, "I am by birth a Genevese (i not set)")
	tester.AssertGetResponse("http://localhost:9080/hello.txt", http.StatusNotFound, "Not found")
	tester.AssertGetResponse("http://localhost:9080/xaccel.php?redir=/hello.txt", http.StatusOK, "Hello")
	tester.AssertGetResponse("http://localhost:9080/xaccel.php?redir=/not_exist.txt", http.StatusNotFound, "")
}

func TestPhpServerXAccelRedirectWorker(t *testing.T) {
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
			order php_server before respond
		}

		localhost:9080 {
			root * ../testdata
			php_server {
				file_server off
				@accel header X-Accel-Redirect *
				handle_response @accel {
					root * ../testdata
				    rewrite {rp.header.X-Accel-Redirect}
				    file_server
				}
			}
			respond "Not found" 404
		}
		`, "caddyfile")

	tester.AssertGetResponse("http://localhost:9080", http.StatusOK, "I am by birth a Genevese (i not set)")
	tester.AssertGetResponse("http://localhost:9080/hello.txt", http.StatusNotFound, "Not found")
	tester.AssertGetResponse("http://localhost:9080/xaccel.php?redir=/hello.txt", http.StatusOK, "Hello")
	tester.AssertGetResponse("http://localhost:9080/xaccel.php?redir=/not_exist.txt", http.StatusNotFound, "")
}
