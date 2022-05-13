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

func TestStartup(t *testing.T) {
	defer frankenphp.Shutdown()
	assert.Nil(t, frankenphp.Startup())
	frankenphp.Shutdown()

	assert.Nil(t, frankenphp.Startup())
}

func setRequestContext(t *testing.T, r *http.Request) *http.Request {
	t.Helper()
	cwd, _ := os.Getwd()

	return frankenphp.NewRequestWithContext(r, cwd+"/testdata/")
}

func TestHelloWorld(t *testing.T) {
	frankenphp.Startup()
	defer frankenphp.Shutdown()

	handler := func(w http.ResponseWriter, r *http.Request) {
		assert.Nil(t, frankenphp.ExecuteScript(w, setRequestContext(t, r)))
	}

	req := httptest.NewRequest("GET", "http://example.com/index.php", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	resp := w.Result()
	body, _ := io.ReadAll(resp.Body)

	assert.Equal(t, "I am by birth a Genevese", string(body))
}

func TestServerVariable(t *testing.T) {
	frankenphp.Startup()
	defer frankenphp.Shutdown()

	handler := func(w http.ResponseWriter, r *http.Request) {
		assert.Nil(t, frankenphp.ExecuteScript(w, setRequestContext(t, r)))
	}

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

func TestPathInfo(t *testing.T) {
	frankenphp.Startup()
	defer frankenphp.Shutdown()

	handler := func(w http.ResponseWriter, r *http.Request) {
		rewriteRequest := setRequestContext(t, r.Clone(context.TODO()))
		rewriteRequest.URL.Path = "/server-variable.php/pathinfo"
		rewriteRequest.Context().Value(frankenphp.FrankenPHPContextKey).(*frankenphp.FrankenPHPContext).Env["REQUEST_URI"] = r.URL.RequestURI()

		assert.Nil(t, frankenphp.ExecuteScript(w, rewriteRequest))
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

func TestHeaders(t *testing.T) {
	frankenphp.Startup()
	defer frankenphp.Shutdown()

	handler := func(w http.ResponseWriter, r *http.Request) {
		assert.Nil(t, frankenphp.ExecuteScript(w, setRequestContext(t, r)))
	}

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

func TestInput(t *testing.T) {
	frankenphp.Startup()
	defer frankenphp.Shutdown()

	handler := func(w http.ResponseWriter, r *http.Request) {
		assert.Nil(t, frankenphp.ExecuteScript(w, setRequestContext(t, r)))
	}

	req := httptest.NewRequest("POST", "http://example.com/input.php", strings.NewReader("post data"))
	w := httptest.NewRecorder()
	handler(w, req)

	resp := w.Result()
	body, _ := io.ReadAll(resp.Body)

	assert.Equal(t, "post data", string(body))
	assert.Equal(t, "bar", resp.Header.Get("Foo"))
}

func TestPostSuperGlobals(t *testing.T) {
	frankenphp.Startup()
	defer frankenphp.Shutdown()

	handler := func(w http.ResponseWriter, r *http.Request) {
		assert.Nil(t, frankenphp.ExecuteScript(w, setRequestContext(t, r)))
	}

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

func TestCookies(t *testing.T) {
	frankenphp.Startup()
	defer frankenphp.Shutdown()

	handler := func(w http.ResponseWriter, r *http.Request) {
		assert.Nil(t, frankenphp.ExecuteScript(w, setRequestContext(t, r)))
	}

	req := httptest.NewRequest("GET", "http://example.com/cookies.php", nil)
	req.AddCookie(&http.Cookie{Name: "foo", Value: "bar"})
	w := httptest.NewRecorder()
	handler(w, req)

	resp := w.Result()
	body, _ := io.ReadAll(resp.Body)

	assert.Contains(t, string(body), "'foo' => 'bar'")
}

func TestSession(t *testing.T) {
	frankenphp.Startup()
	defer frankenphp.Shutdown()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Nil(t, frankenphp.ExecuteScript(w, setRequestContext(t, r)))
	}))
	defer ts.Close()

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
	assert.Equal(t, "Count: 0\n", string(body1))

	resp2, err := client.Get(ts.URL + "/session.php")
	if err != nil {
		panic(err)
	}

	body2, _ := io.ReadAll(resp2.Body)
	assert.Equal(t, "Count: 1\n", string(body2))
}

func TestPhpInfo(t *testing.T) {
	frankenphp.Startup()
	defer frankenphp.Shutdown()

	handler := func(w http.ResponseWriter, r *http.Request) {
		assert.Nil(t, frankenphp.ExecuteScript(w, setRequestContext(t, r)))
	}

	req := httptest.NewRequest("GET", "http://example.com/phpinfo.php", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	resp := w.Result()
	body, _ := io.ReadAll(resp.Body)

	assert.Contains(t, string(body), "frankenphp")
}
