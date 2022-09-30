package caddy_test

import (
	"fmt"
	"net/http"
	"sync"
	"testing"

	"github.com/caddyserver/caddy/v2/caddytest"
)

func TestPHP(t *testing.T) {
	var wg sync.WaitGroup
	caddytest.Default.AdminPort = 2019
	tester := caddytest.NewTester(t)
	tester.InitServer(`
		{
			admin localhost:2999
			http_port 9080
			https_port 9443
		}

		localhost:9080 {
			respond "Hello"
		}
		`, "caddyfile")

	wg.Add(100)
	for i := 0; i < 100; i++ {
		go func(i int) {
			tester.AssertGetResponse(fmt.Sprintf("http://localhost:9080?i=%d", i), http.StatusOK, fmt.Sprintf("I am by birth a Genevese (%d)", i))
			wg.Done()
		}(i)
	}
	wg.Wait()
}

func TestAutoHTTPtoHTTPSRedirectsImplicitPort(t *testing.T) {
	tester := caddytest.NewTester(t)
	tester.InitServer(`
	{
		http_port     9080
		https_port    9443
	}
	localhost
	respond "Yahaha! You found me!"
  `, "caddyfile")

	tester.AssertRedirect("http://localhost:9080/", "https://localhost/", http.StatusPermanentRedirect)
}
