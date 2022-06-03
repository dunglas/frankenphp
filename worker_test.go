package frankenphp_test

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/dunglas/frankenphp"
	"github.com/stretchr/testify/assert"
)

func TestWorker(t *testing.T) {
	runTest(t, func(handler func(http.ResponseWriter, *http.Request), _ *httptest.Server, i int) {
		formData := url.Values{"baz": {"bat"}}
		req := httptest.NewRequest("POST", "http://example.com/worker.php?foo=bar", strings.NewReader(formData.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()
		handler(w, req)

		resp := w.Result()
		body, _ := io.ReadAll(resp.Body)

		assert.Contains(t, string(body), fmt.Sprintf("Requests handled: %d", i*2))

		formData2 := url.Values{"baz2": {"bat2"}}
		req2 := httptest.NewRequest("POST", "http://example.com/worker.php?foo2=bar2", strings.NewReader(formData2.Encode()))
		req2.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		w2 := httptest.NewRecorder()
		handler(w2, req2)

		resp2 := w2.Result()
		body2, _ := io.ReadAll(resp2.Body)

		assert.Contains(t, string(body2), fmt.Sprintf("Requests handled: %d", i*2+1))
	}, &testOptions{workerScript: "worker.php", nbWorkers: 1, nbParrallelRequests: 1})
}

func ExampleWorkerHandleRequest() {
	frankenphp.StartWorkers("worker.php", 5)

	phpHandler := func(w http.ResponseWriter, req *http.Request) {
		if err := frankenphp.WorkerHandleRequest(w, req); err != nil {
			log.Print(fmt.Errorf("error executing PHP script: %w", err))
		}
	}

	http.HandleFunc("/", phpHandler)
	log.Fatal(http.ListenAndServe(":8080", nil))
}
