package frankenphp_test

import (
	"context"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/dunglas/frankenphp"
	"github.com/stretchr/testify/assert"
)

func setRequestContext(t *testing.T, r *http.Request) *http.Request {
	t.Helper()

	cwd, _ := os.Getwd()

	return frankenphp.NewRequestWithContext(r, cwd+"/testdata/")
}

func createTestHandler(t *testing.T, workerScript string) (shutdown func(), handler func(http.ResponseWriter, *http.Request), iterations int) {
	assert.Nil(t, frankenphp.Startup())

	if workerScript == "" {
		iterations = 1
	} else {
		iterations = 2
		cwd, _ := os.Getwd()

		frankenphp.StartWorkers(cwd+"/testdata/"+workerScript, 1)
	}

	shutdown = func() {
		if workerScript != "" {
			frankenphp.StopWorkers()
		}
		frankenphp.Shutdown()
	}

	handler = func(w http.ResponseWriter, r *http.Request) {
		var err error

		req := setRequestContext(t, r)
		if workerScript == "" {
			err = frankenphp.ExecuteScript(w, req)
		} else {
			err = frankenphp.WorkerHandleRequest(w, req)
		}

		assert.Nil(t, err)
	}

	return
}

func TestStartup(t *testing.T) {
	defer frankenphp.Shutdown()
	assert.Nil(t, frankenphp.Startup())
	frankenphp.Shutdown()

	assert.Nil(t, frankenphp.Startup())
}

func TestHelloWorld_module(t *testing.T) { testHelloWorld(t, "") }
func TestHelloWorld_worker(t *testing.T) { testHelloWorld(t, "index.php") }
func testHelloWorld(t *testing.T, scriptName string) {
	shutdown, handler, iterations := createTestHandler(t, scriptName)
	defer shutdown()

	for i := 0; i < iterations; i++ {
		req := httptest.NewRequest("GET", "http://example.com/index.php", nil)
		w := httptest.NewRecorder()
		handler(w, req)

		resp := w.Result()
		body, _ := io.ReadAll(resp.Body)

		assert.Equal(t, "I am by birth a Genevese", string(body))
	}
}

func TestServerVariable_module(t *testing.T) { testServerVariable(t, "") }
func TestServerVariable_worker(t *testing.T) {
	testServerVariable(t, "server-variable.php")
}
func testServerVariable(t *testing.T, scriptName string) {
	shutdown, handler, iterations := createTestHandler(t, scriptName)
	defer shutdown()

	for i := 0; i < iterations; i++ {
		req := httptest.NewRequest("GET", "http://example.com/server-variable.php?foo=a&bar=b#hash", nil)
		req.SetBasicAuth("kevin", "password")
		w := httptest.NewRecorder()
		handler(w, req)

		resp := w.Result()
		body, _ := io.ReadAll(resp.Body)

		strBody := string(body)

		assert.Contains(t, strBody, "[REMOTE_HOST]")
		assert.Contains(t, strBody, "[REMOTE_USER] => kevin")
		assert.Contains(t, strBody, "[PHP_AUTH_USER] => kevin")
		assert.Contains(t, strBody, "[PHP_AUTH_PW] => password")
		assert.Contains(t, strBody, "[HTTP_AUTHORIZATION] => Basic a2V2aW46cGFzc3dvcmQ=")
		assert.Contains(t, strBody, "[DOCUMENT_ROOT]")
		assert.Contains(t, strBody, "[CONTENT_TYPE]")
		assert.Contains(t, strBody, "[QUERY_STRING] => foo=a&bar=b#hash")
		assert.Contains(t, strBody, "[REQUEST_URI] => /server-variable.php?foo=a&bar=b#hash")
		assert.Contains(t, strBody, "[SCRIPT_NAME]")
		assert.Contains(t, strBody, "[CONTENT_LENGTH]")
		assert.Contains(t, strBody, "[REMOTE_ADDR]")
		assert.Contains(t, strBody, "[REMOTE_PORT]")
		assert.Contains(t, strBody, "[REQUEST_SCHEME] => http")
		assert.Contains(t, strBody, "[DOCUMENT_URI]")
		assert.Contains(t, strBody, "[AUTH_TYPE]")
		assert.Contains(t, strBody, "[REMOTE_IDENT]")
		assert.Contains(t, strBody, "[REQUEST_METHOD] => GET")
		assert.Contains(t, strBody, "[SERVER_NAME] => example.com")
		assert.Contains(t, strBody, "[SERVER_PROTOCOL] => HTTP/1.1")
		assert.Contains(t, strBody, "[SCRIPT_FILENAME]")
		assert.Contains(t, strBody, "[SERVER_SOFTWARE] => FrankenPHP")
		assert.Contains(t, strBody, "[REQUEST_TIME_FLOAT]")
		assert.Contains(t, strBody, "[REQUEST_TIME]")
		assert.Contains(t, strBody, "[REQUEST_TIME]")
	}
}

func TestPathInfo_module(t *testing.T) { testPathInfo(t, "") }
func TestPathInfo_worker(t *testing.T) {
	testPathInfo(t, "server-variable.php")
}
func testPathInfo(t *testing.T, scriptName string) {
	shutdown, _, iterations := createTestHandler(t, scriptName)
	defer shutdown()

	for i := 0; i < iterations; i++ {
		handler := func(w http.ResponseWriter, r *http.Request) {
			rewriteRequest := setRequestContext(t, r.Clone(context.TODO()))
			rewriteRequest.URL.Path = "/server-variable.php/pathinfo"
			fc, _ := frankenphp.FromContext(rewriteRequest.Context())
			fc.Env["REQUEST_URI"] = r.URL.RequestURI()

			if scriptName == "" {
				assert.Nil(t, frankenphp.ExecuteScript(w, rewriteRequest))
			} else {
				assert.Nil(t, frankenphp.WorkerHandleRequest(w, rewriteRequest))
			}
		}

		req := httptest.NewRequest("GET", "http://example.com/pathinfo", nil)
		w := httptest.NewRecorder()
		handler(w, req)

		resp := w.Result()
		body, _ := io.ReadAll(resp.Body)

		strBody := string(body)

		assert.Contains(t, strBody, "[PATH_INFO] => /pathinfo")
		assert.Contains(t, strBody, "[REQUEST_URI] => /pathinfo")
		assert.Contains(t, strBody, "[PATH_TRANSLATED] =>")
		assert.Contains(t, strBody, "[SCRIPT_NAME] => /server-variable.php")
	}
}

func TestHeaders_module(t *testing.T) { testHeaders(t, "") }
func TestHeaders_worker(t *testing.T) { testHeaders(t, "headers.php") }
func testHeaders(t *testing.T, scriptName string) {
	shutdown, handler, iterations := createTestHandler(t, scriptName)
	defer shutdown()

	for i := 0; i < iterations; i++ {
		req := httptest.NewRequest("GET", "http://example.com/headers.php", nil)
		w := httptest.NewRecorder()
		handler(w, req)

		resp := w.Result()
		body, _ := io.ReadAll(resp.Body)

		assert.Equal(t, "Hello", string(body))
		assert.Equal(t, 201, resp.StatusCode)
		assert.Equal(t, "bar", resp.Header.Get("Foo"))
		assert.Equal(t, "bar2", resp.Header.Get("Foo2"))
	}
}

func TestInput_module(t *testing.T) { testInput(t, "") }
func TestInput_worker(t *testing.T) { testInput(t, "input.php") }
func testInput(t *testing.T, scriptName string) {
	shutdown, handler, iterations := createTestHandler(t, scriptName)
	defer shutdown()

	for i := 0; i < iterations; i++ {
		req := httptest.NewRequest("POST", "http://example.com/input.php", strings.NewReader("post data"))
		w := httptest.NewRecorder()
		handler(w, req)

		resp := w.Result()
		body, _ := io.ReadAll(resp.Body)

		assert.Equal(t, "post data", string(body))
		assert.Equal(t, "bar", resp.Header.Get("Foo"))
	}
}

func TestPostSuperGlobals_module(t *testing.T) { testPostSuperGlobals(t, "") }
func TestPostSuperGlobals_worker(t *testing.T) { testPostSuperGlobals(t, "super-globals.php") }
func testPostSuperGlobals(t *testing.T, scriptName string) {
	shutdown, handler, iterations := createTestHandler(t, scriptName)
	defer shutdown()

	for i := 0; i < iterations; i++ {
		formData := url.Values{"baz": {"bat"}}
		req := httptest.NewRequest("POST", "http://example.com/super-globals.php?foo=bar", strings.NewReader(formData.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()
		handler(w, req)

		resp := w.Result()
		body, _ := io.ReadAll(resp.Body)

		assert.Contains(t, string(body), "'foo' => 'bar'")
		assert.Contains(t, string(body), "'baz' => 'bat'")
	}
}

func TestCookies_module(t *testing.T) { testCookies(t, "") }
func TestCookies_worker(t *testing.T) { testCookies(t, "cookies.php") }
func testCookies(t *testing.T, scriptName string) {
	shutdown, handler, iterations := createTestHandler(t, scriptName)
	defer shutdown()

	for i := 0; i < iterations; i++ {
		req := httptest.NewRequest("GET", "http://example.com/cookies.php", nil)
		req.AddCookie(&http.Cookie{Name: "foo", Value: "bar"})
		w := httptest.NewRecorder()
		handler(w, req)

		resp := w.Result()
		body, _ := io.ReadAll(resp.Body)

		assert.Contains(t, string(body), "'foo' => 'bar'")
	}
}

func TestSession_module(t *testing.T) { testSession(t, "") }
func TestSession_worker(t *testing.T) {
	testSession(t, "session.php")
}
func testSession(t *testing.T, scriptName string) {
	shutdown, handler, iterations := createTestHandler(t, scriptName)
	defer shutdown()

	ts := httptest.NewServer(http.HandlerFunc(handler))
	defer ts.Close()

	for i := 0; i < iterations; i++ {
		jar, err := cookiejar.New(&cookiejar.Options{})
		if err != nil {
			panic(err)
		}

		client := &http.Client{Jar: jar}

		resp1, err := client.Get(ts.URL + "/session.php")
		if err != nil {
			panic(err)
		}

		body1, _ := io.ReadAll(resp1.Body)
		t.Log(string(body1))
		assert.Equal(t, "Count: 0\n", string(body1))

		resp2, err := client.Get(ts.URL + "/session.php")
		if err != nil {
			panic(err)
		}

		body2, _ := io.ReadAll(resp2.Body)
		assert.Equal(t, "Count: 1\n", string(body2))
	}
}

func TestPhpInfo_module(t *testing.T) { testPhpInfo(t, "") }
func TestPhpInfo_worker(t *testing.T) { testPhpInfo(t, "phpinfo.php") }
func testPhpInfo(t *testing.T, scriptName string) {
	shutdown, handler, iterations := createTestHandler(t, scriptName)
	defer shutdown()

	for i := 0; i < iterations; i++ {
		req := httptest.NewRequest("GET", "http://example.com/phpinfo.php", nil)
		w := httptest.NewRecorder()
		handler(w, req)

		resp := w.Result()
		body, _ := io.ReadAll(resp.Body)

		assert.Contains(t, string(body), "frankenphp")
	}
}
