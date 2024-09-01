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


func TestWorkerWithWatcher(t *testing.T) {
	updateTestFile("./testdata/files/test.json", "JsonFile1")
    updateTestFile("./testdata/files/test.txt", "TextFile1")

	runTest(t, func(handler func(http.ResponseWriter, *http.Request), _ *httptest.Server, i int) {
		// first we verify that the worker is working correctly
		body := fetchBody("GET", "http://example.com/worker-with-watcher.php", handler)
		assert.Equal(t, "JsonFile1TextFile1", body)

		// now we verify that updating a .txt file does not cause a reload
		updateTestFile("./testdata/files/test.txt", "TextFile2")
		time.Sleep(1000 * time.Millisecond)
		body = fetchBody("GET", "http://example.com/worker-with-watcher.php", handler)
		assert.Equal(t, "JsonFile1TextFile1", body)

		// lastly we verify that updating a .json file does cause a reload
		updateTestFile("./testdata/files/test.json", "JsonFile2")
		time.Sleep(1000 * time.Millisecond)
        body = fetchBody("GET", "http://example.com/worker-with-watcher.php", handler)
		assert.Equal(t, "JsonFile2TextFile2", body)

	}, &testOptions{nbParrallelRequests: 1, nbWorkers: 1, workerScript: "worker-with-watcher.php", watch:"./testdata/**/*.json"})
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