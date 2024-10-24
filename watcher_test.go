//go:build !nowatcher

package frankenphp_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// we have to wait a few milliseconds for the watcher debounce to take effect
const pollingTime = 250

// in tests checking for no reload: we will poll 3x250ms = 0.75s
const minTimesToPollForChanges = 3

// in tests checking for a reload: we will poll a maximum of 60x250ms = 15s
const maxTimesToPollForChanges = 60

func TestWorkersShouldReloadOnMatchingPattern(t *testing.T) {
	watch := []string{"./testdata/**/*.txt"}

	runTest(t, func(handler func(http.ResponseWriter, *http.Request), _ *httptest.Server, i int) {
		requestBodyHasReset := pollForWorkerReset(t, handler, maxTimesToPollForChanges)
		assert.True(t, requestBodyHasReset)
	}, &testOptions{nbParrallelRequests: 1, nbWorkers: 1, workerScript: "worker-with-watcher.php", watch: watch})
}

func TestWorkersShouldNotReloadOnExcludingPattern(t *testing.T) {
	watch := []string{"./testdata/**/*.php"}

	runTest(t, func(handler func(http.ResponseWriter, *http.Request), _ *httptest.Server, i int) {
		requestBodyHasReset := pollForWorkerReset(t, handler, minTimesToPollForChanges)
		assert.False(t, requestBodyHasReset)
	}, &testOptions{nbParrallelRequests: 1, nbWorkers: 1, workerScript: "worker-with-watcher.php", watch: watch})
}

func fetchBody(method string, url string, handler func(http.ResponseWriter, *http.Request)) string {
	req := httptest.NewRequest(method, url, nil)
	w := httptest.NewRecorder()
	handler(w, req)
	resp := w.Result()
	body, _ := io.ReadAll(resp.Body)

	return string(body)
}

func pollForWorkerReset(t *testing.T, handler func(http.ResponseWriter, *http.Request), limit int) bool {
	// first we make an initial request to start the request counter
	body := fetchBody("GET", "http://example.com/worker-with-watcher.php", handler)
	assert.Equal(t, "requests:1", body)

	// now we spam file updates and check if the request counter resets
	for i := 0; i < limit; i++ {
		updateTestFile("./testdata/files/test.txt", "updated", t)
		time.Sleep(pollingTime * time.Millisecond)
		body := fetchBody("GET", "http://example.com/worker-with-watcher.php", handler)
		if body == "requests:1" {
			return true
		}
	}
	return false
}

func updateTestFile(fileName string, content string, t *testing.T) {
	absFileName, err := filepath.Abs(fileName)
	assert.NoError(t, err)
	dirName := filepath.Dir(absFileName)
	if _, err := os.Stat(dirName); os.IsNotExist(err) {
		err = os.MkdirAll(dirName, 0700)
		assert.NoError(t, err)
	}
	bytes := []byte(content)
	err = os.WriteFile(absFileName, bytes, 0644)
	assert.NoError(t, err)
}
