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
	tester := caddytest.NewTester(t)
	tester.InitServer(`
		{
			http_port     9080
			https_port    9443

			#frankenphp
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

				# Handle PHP files with FrankenPHP
				@phpFiles path *.php
				php @phpFiles
		
				respond 404
			}
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
