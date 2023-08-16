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
		"Request body size: 1048576 (undefined)",
	)
}
