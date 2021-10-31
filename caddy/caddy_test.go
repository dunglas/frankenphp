package caddy_test

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
			root * {env.PWD}/../testdata
			# Add trailing slash for directory requests
			@canonicalPath {
			    file {path}/index.php
			    not path */
			}
			redir @canonicalPath {path}/ 308

			# If the requested file does not exist, try index files
			@indexFiles file {
			    try_files {path} {path}/index.php index.php
			    split_path .php
			}
			rewrite @indexFiles {http.matchers.file.relative}

			# Proxy PHP files to the FastCGI responder
			@phpFiles path *.php
			php @phpFiles
	
			respond 404
		}
	}
	`, "caddyfile")

	tester.AssertGetResponse("http://localhost:9080", http.StatusOK, "I am by birth a Genevese")
}
