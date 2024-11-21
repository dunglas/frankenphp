package frankenphp_test

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAddToMemoryCache(t *testing.T) {
	runTest(t, func(handler func(http.ResponseWriter, *http.Request), _ *httptest.Server, i int) {
		body := fetchBody("GET", "http://example.com/memory-cache.php", handler)
		assert.Equal(t, "nothing", body)

		body = fetchBody("GET", "http://example.com/memory-cache.php?remember=value", handler)
		assert.Equal(t, "value", body)
	}, &testOptions{nbParrallelRequests: 1})
}

func TestRemoveFromMemoryCache(t *testing.T) {
	runTest(t, func(handler func(http.ResponseWriter, *http.Request), _ *httptest.Server, i int) {
		body := fetchBody("GET", "http://example.com/memory-cache.php?remember=value", handler)
		assert.Equal(t, "value", body)

		body = fetchBody("GET", "http://example.com/memory-cache.php?forget=value", handler)
		assert.Equal(t, "nothing", body)
	}, &testOptions{workerScript: "memory-cache.php", nbWorkers: 1, nbParrallelRequests: 1})
}

func TestReadFromMemoryCacheInParallel(t *testing.T) {
	numberOfWorkers := 10
	requestsPerWorker := 10
	runTest(t, func(handler func(http.ResponseWriter, *http.Request), _ *httptest.Server, i int) {
		body := fetchBody("GET", "http://example.com/memory-cache.php?remember=value", handler)
		assert.Equal(t, "value", body)

		// Test that all threads have the memory cache
		wg := &sync.WaitGroup{}
		wg.Add(numberOfWorkers * requestsPerWorker)
		for i := 0; i < numberOfWorkers*requestsPerWorker; i++ {
			go func() {
				body := fetchBody("GET", "http://example.com/memory-cache.php", handler)
				assert.Equal(t, "value", body)

				wg.Done()
			}()
		}
		wg.Wait()
	}, &testOptions{workerScript: "memory-cache.php", nbWorkers: numberOfWorkers, nbParrallelRequests: 1})
}

func WriteToMemoryCacheInParallel(t *testing.T) {
	numberOfWorkers := 10
	requestsPerWorker := 100
	runTest(t, func(handler func(http.ResponseWriter, *http.Request), _ *httptest.Server, i int) {
		for j := 0; j < requestsPerWorker; j++ {
			value := "worker:" + strconv.Itoa(i) + ", request:" + strconv.Itoa(j)
			body := fetchBody("GET", "http://example.com/memory-cache.php?remember="+value+"key="+value, handler)
			assert.Equal(t, value, body)
		}
	}, &testOptions{workerScript: "memory-cache.php", nbWorkers: numberOfWorkers, nbParrallelRequests: 10})
}
