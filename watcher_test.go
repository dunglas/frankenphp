package frankenphp_test

import (
	"github.com/stretchr/testify/assert"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
	"github.com/dunglas/frankenphp"
)

// we have to wait a few milliseconds for the watcher debounce to take effect
const pollingTime = 150
const minTimesToPollForChanges = 5
const maxTimesToPollForChanges = 100 // we will poll a maximum of 100x150ms = 15s

func TestWorkersShouldReloadOnMatchingPattern(t *testing.T) {
	const filePattern = "./testdata/**/*.txt"
	watchOptions := []frankenphp.WatchOption{frankenphp.WithWatcherShortForm(filePattern)}

	runTest(t, func(handler func(http.ResponseWriter, *http.Request), _ *httptest.Server, i int) {
		requestBodyHasReset := pollForWorkerReset(t, handler, maxTimesToPollForChanges)
		assert.True(t, requestBodyHasReset)
	}, &testOptions{nbParrallelRequests: 1, nbWorkers: 1, workerScript: "worker-with-watcher.php", watchOptions: watchOptions})
}

func TestWorkersShouldNotReloadOnExcludingPattern(t *testing.T) {
	const filePattern = "./testdata/**/*.php"
	watchOptions := []frankenphp.WatchOption{frankenphp.WithWatcherShortForm(filePattern)}

	runTest(t, func(handler func(http.ResponseWriter, *http.Request), _ *httptest.Server, i int) {
		requestBodyHasReset := pollForWorkerReset(t, handler, minTimesToPollForChanges)
		assert.False(t, requestBodyHasReset)
	}, &testOptions{nbParrallelRequests: 1, nbWorkers: 1, workerScript: "worker-with-watcher.php", watchOptions: watchOptions})
}

func TestWorkersReloadOnMatchingIncludedRegex(t *testing.T) {
	const include = "\\.txt$"
	watchOptions := []frankenphp.WatchOption{
		frankenphp.WithWatcherDirs([]string{"./testdata"}),
		frankenphp.WithWatcherRecursion(true),
		frankenphp.WithWatcherFilters(include, "", true, false),
	}

	runTest(t, func(handler func(http.ResponseWriter, *http.Request), _ *httptest.Server, i int) {
		requestBodyHasReset := pollForWorkerReset(t, handler, maxTimesToPollForChanges)
		assert.True(t, requestBodyHasReset)
	}, &testOptions{nbParrallelRequests: 1, nbWorkers: 1, workerScript: "worker-with-watcher.php", watchOptions: watchOptions})
}

func TestWorkersDoNotReloadOnExcludingRegex(t *testing.T) {
	const exclude ="\\.txt$"
	watchOptions := []frankenphp.WatchOption{
		frankenphp.WithWatcherDirs([]string{"./testdata"}),
		frankenphp.WithWatcherRecursion(true),
		frankenphp.WithWatcherFilters("", exclude, false, false),
	}

	runTest(t, func(handler func(http.ResponseWriter, *http.Request), _ *httptest.Server, i int) {
		requestBodyHasReset := pollForWorkerReset(t, handler, minTimesToPollForChanges)
		assert.False(t, requestBodyHasReset)
	}, &testOptions{nbParrallelRequests: 1, nbWorkers: 1, workerScript: "worker-with-watcher.php", watchOptions: watchOptions})
}

func TestWorkerShouldReloadUsingPolling(t *testing.T) {
	watchOptions := []frankenphp.WatchOption{
		frankenphp.WithWatcherDirs([]string{"./testdata/files"}),
		frankenphp.WithWatcherMonitorType("poll"),
	}

	runTest(t, func(handler func(http.ResponseWriter, *http.Request), _ *httptest.Server, i int) {
		requestBodyHasReset := pollForWorkerReset(t, handler, maxTimesToPollForChanges)
		assert.True(t, requestBodyHasReset)
	}, &testOptions{nbParrallelRequests: 1, nbWorkers: 1, workerScript: "worker-with-watcher.php", watchOptions: watchOptions})
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
