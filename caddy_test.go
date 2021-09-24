package frankenphp_test

import (
	"net/http"
	"testing"

	"github.com/caddyserver/caddy/v2/caddytest"
)

func TestPHP(t *testing.T) {
	tester := caddytest.NewTester(t)
	tester.InitServer(`
	{
		http_port     9080
		https_port    9443
	}
	localhost:9080 {
		route {
			php
	
			respond 404
		}
	}
	`, "caddyfile")

	tester.AssertGetResponse("http://localhost:9080", http.StatusOK, "Hello World")
}
