package frankenphp_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/dunglas/frankenphp"
	"github.com/stretchr/testify/assert"
)

func TestWorker(t *testing.T) {
	defer frankenphp.Shutdown()

	handler := func(w http.ResponseWriter, r *http.Request) {
		assert.Nil(t, frankenphp.WorkerHandleRequest(w, setRequestContext(t, r)))
		//assert.Nil(t, frankenphp.ExecuteScript(w, setRequestContext(t, r)))
	}

	formData := url.Values{"baz": {"bat"}}
	req := httptest.NewRequest("POST", "http://example.com/worker.php?foo=bar", strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	handler(w, req)

	resp := w.Result()
	body, _ := io.ReadAll(resp.Body)

	assert.Contains(t, string(body), "'foo' => 'bar'")
	//assert.Contains(t, string(body), "'baz' => 'bat'")

	req2 := httptest.NewRequest("GET", "http://example.com/worker.php?foo2=bar2", nil)
	w2 := httptest.NewRecorder()
	handler(w2, req2)

	resp2 := w.Result()
	body2, _ := io.ReadAll(resp2.Body)

	assert.Contains(t, string(body2), "'foo2' => 'bar2'")

}
