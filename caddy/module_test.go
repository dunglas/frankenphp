package caddy_test

import (
	"net/http"
	"os"
	"strconv"
	"testing"

	"github.com/caddyserver/caddy/v2/caddytest"
)

func TestRootBehavesTheSameOutsideAndInsidePhpServer(t *testing.T) {
	tester := caddytest.NewTester(t)
	testPortNum, _ := strconv.Atoi(testPort)
	testPortTwo := strconv.Itoa(testPortNum + 1)
	expectedFileResponse, _ := os.ReadFile("../testdata/files/static.txt")
	tester.InitServer(`
		{
			skip_install_trust
			admin localhost:2999
		}

		http://localhost:`+testPort+` {
			root ../testdata/files
			php_server
		}

		http://localhost:`+testPortTwo+` {
			php_server {
				root ../testdata/files
			}
		}
		`, "caddyfile")

	// serve the file with root outside of php_server
	tester.AssertGetResponse("http://localhost:"+testPort+"/static.txt", http.StatusOK, string(expectedFileResponse))

	// serve the file with root within php_server
	tester.AssertGetResponse("http://localhost:"+testPortTwo+"/static.txt", http.StatusOK, string(expectedFileResponse))
}
