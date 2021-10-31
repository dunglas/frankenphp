package frankenphp_test

import (
	"context"
	"io"
	"net/http"
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
}

func TestHelloWorld(t *testing.T) {
	defer frankenphp.Shutdown()

	f := frankenphp.NewFrankenPHP()
	handler := func(w http.ResponseWriter, r *http.Request) {
		cwd, _ := os.Getwd()
		assert.Nil(t, f.ExecuteScript(cwd+"/testdata/", w, r, nil))
	}

	req := httptest.NewRequest("GET", "http://example.com/index.php", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	resp := w.Result()
	body, _ := io.ReadAll(resp.Body)

	assert.Equal(t, "I am by birth a Genevese", string(body))
}

func TestServerVariable(t *testing.T) {
	defer frankenphp.Shutdown()

	f := frankenphp.NewFrankenPHP()
	handler := func(w http.ResponseWriter, r *http.Request) {
		cwd, _ := os.Getwd()
		assert.Nil(t, f.ExecuteScript(cwd+"/testdata/", w, r, nil))
	}

	req := httptest.NewRequest("GET", "http://kevin:password@example.com/server-variable.php?foo=a&bar=b#hash", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	resp := w.Result()
	body, _ := io.ReadAll(resp.Body)

	strBody := string(body)

	assert.Contains(t, strBody, "[REMOTE_HOST]")
	assert.Contains(t, strBody, "[REMOTE_USER]")
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
	defer frankenphp.Shutdown()

	f := frankenphp.NewFrankenPHP()
	handler := func(w http.ResponseWriter, r *http.Request) {
		cwd, _ := os.Getwd()

		rewriteRequest := r.Clone(context.TODO())
		rewriteRequest.URL.Path = "/server-variable.php/pathinfo"

		assert.Nil(t, f.ExecuteScript(cwd+"/testdata/", w, rewriteRequest, r))
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
	defer frankenphp.Shutdown()

	f := frankenphp.NewFrankenPHP()
	handler := func(w http.ResponseWriter, r *http.Request) {
		cwd, _ := os.Getwd()
		assert.Nil(t, f.ExecuteScript(cwd+"/testdata/", w, r, nil))
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
	defer frankenphp.Shutdown()

	f := frankenphp.NewFrankenPHP()
	handler := func(w http.ResponseWriter, r *http.Request) {
		cwd, _ := os.Getwd()
		assert.Nil(t, f.ExecuteScript(cwd+"/testdata/", w, r, nil))
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
	defer frankenphp.Shutdown()

	f := frankenphp.NewFrankenPHP()
	handler := func(w http.ResponseWriter, r *http.Request) {
		cwd, _ := os.Getwd()
		assert.Nil(t, f.ExecuteScript(cwd+"/testdata/", w, r, nil))
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
	defer frankenphp.Shutdown()

	f := frankenphp.NewFrankenPHP()
	handler := func(w http.ResponseWriter, r *http.Request) {
		cwd, _ := os.Getwd()
		assert.Nil(t, f.ExecuteScript(cwd+"/testdata/", w, r, nil))
	}

	req := httptest.NewRequest("GET", "http://example.com/cookies.php", nil)
	req.AddCookie(&http.Cookie{Name: "foo", Value: "bar"})
	w := httptest.NewRecorder()
	handler(w, req)

	resp := w.Result()
	body, _ := io.ReadAll(resp.Body)

	assert.Contains(t, string(body), "'foo' => 'bar'")
}
