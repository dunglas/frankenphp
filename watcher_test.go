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


func TestWorkerShouldReloadOnMatchingPattern(t *testing.T) {
	const filePattern = "./testdata/**/*.txt"
	updateTestFile("./testdata/files/test.txt", "version1")

	runTest(t, func(handler func(http.ResponseWriter, *http.Request), _ *httptest.Server, i int) {
		// first we verify that the worker is working correctly
		body := fetchBody("GET", "http://example.com/worker-with-watcher.php", handler)
		assert.Equal(t, "version1", body)

		// now we verify that updating a .txt file does not cause a reload
		updateTestFile("./testdata/files/test.txt", "version2")
		time.Sleep(1000 * time.Millisecond)
		body = fetchBody("GET", "http://example.com/worker-with-watcher.php", handler)
		assert.Equal(t, "version2", body)

	}, &testOptions{nbParrallelRequests: 1, nbWorkers: 1, workerScript: "worker-with-watcher.php", watch:filePattern})
}

func TestWorkerShouldNotReloadOnNonMatchingPattern(t *testing.T) {
	const filePattern = "./testdata/**/*.json"
	updateTestFile("./testdata/files/test.txt", "version1")

	runTest(t, func(handler func(http.ResponseWriter, *http.Request), _ *httptest.Server, i int) {
		// first we verify that the worker is working correctly
		body := fetchBody("GET", "http://example.com/worker-with-watcher.php", handler)
		assert.Equal(t, "version1", body)

		// now we verify that updating a .txt file does not cause a reload
		updateTestFile("./testdata/files/test.txt", "version2")
		time.Sleep(1000 * time.Millisecond)
		body = fetchBody("GET", "http://example.com/worker-with-watcher.php", handler)
		assert.Equal(t, "version1", body)

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