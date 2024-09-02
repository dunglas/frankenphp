package frankenphp_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"os"
	"time"
	"github.com/stretchr/testify/assert"
)

// we have to wait a few milliseconds for the worker debounce to take effect
const debounceMilliseconds = 500


func TestWorkerShouldReloadOnMatchingPattern(t *testing.T) {
	const filePattern = "./testdata/**/*.txt"

	runTest(t, func(handler func(http.ResponseWriter, *http.Request), _ *httptest.Server, i int) {
		// first we verify that the worker is working correctly
		body := fetchBody("GET", "http://example.com/worker-with-watcher.php", handler)
		assert.Equal(t, "requests:1", body)

		// now we verify that updating a .txt file does not cause a reload
		updateTestFile("./testdata/files/test.txt", "updated")
		time.Sleep(debounceMilliseconds * time.Millisecond)
		body = fetchBody("GET", "http://example.com/worker-with-watcher.php", handler)
		assert.Equal(t, "requests:1", body)

	}, &testOptions{nbParrallelRequests: 1, nbWorkers: 1, workerScript: "worker-with-watcher.php", watch:filePattern})
}

func TestWorkerShouldNotReloadOnNonMatchingPattern(t *testing.T) {
	const filePattern = "./testdata/**/*.txt"

	runTest(t, func(handler func(http.ResponseWriter, *http.Request), _ *httptest.Server, i int) {
		// first we verify that the worker is working correctly
		body := fetchBody("GET", "http://example.com/worker-with-watcher.php", handler)
		assert.Equal(t, "requests:1", body)

		// now we verify that updating a .json file does not cause a reload
		updateTestFile("./testdata/files/test.json", "{updated:true}")
		time.Sleep(debounceMilliseconds * time.Millisecond)
		body = fetchBody("GET", "http://example.com/worker-with-watcher.php", handler)
		assert.Equal(t, "requests:2", body)

	}, &testOptions{nbParrallelRequests: 1, nbWorkers: 1, workerScript: "worker-with-watcher.php", watch:filePattern})
}


func fetchBody(method string, url string, handler func(http.ResponseWriter, *http.Request)) string {
	req := httptest.NewRequest(method, url, nil)
	w := httptest.NewRecorder()
	handler(w, req)
	resp := w.Result()
	body, _ := io.ReadAll(resp.Body)

	return string(body)
}

func updateTestFile(fileName string, content string){
	bytes := []byte(content)
	err := os.WriteFile(fileName, bytes, 0644)
	if(err != nil) {
		panic(err)
	}
}