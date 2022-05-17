package frankenphp_test

import (
	"io"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWorker(t *testing.T) {
	shutdown, handler, _ := createTestHandler(t, "worker.php")
	defer shutdown()

	/*for i := 0; i < iterations; i++ {*/
	formData := url.Values{"baz": {"bat"}}
	req := httptest.NewRequest("POST", "http://example.com/worker.php?foo=bar", strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	handler(w, req)

	resp := w.Result()
	body, _ := io.ReadAll(resp.Body)

	t.Log(string(body))

	assert.Contains(t, string(body), "Requests handled: 0")

	formData2 := url.Values{"baz2": {"bat2"}}
	req2 := httptest.NewRequest("POST", "http://example.com/worker.php?foo2=bar2", strings.NewReader(formData2.Encode()))
	req2.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	w2 := httptest.NewRecorder()
	handler(w2, req2)

	resp2 := w2.Result()
	body2, _ := io.ReadAll(resp2.Body)

	t.Log(string(body2))

	assert.Contains(t, string(body2), "Requests handled: 1")
	/*}*/
}
