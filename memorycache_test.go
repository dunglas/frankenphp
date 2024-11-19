package frankenphp_test

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAddToMemoryCache(t *testing.T) {
	runTest(t, func(handler func(http.ResponseWriter, *http.Request), _ *httptest.Server, i int) {
        body := fetchBody("GET", "http://example.com/memory-cache.php", handler)
        assert.Equal(t, body, "nothing")

        body = fetchBody("GET", "http://example.com/memory-cache.php?remember=value", handler)
        assert.Equal(t, body, "value")
	}, nil)
}

func TestRemoveFromMemoryCache(t *testing.T) {
	runTest(t, func(handler func(http.ResponseWriter, *http.Request), _ *httptest.Server, i int) {
        body := fetchBody("GET", "http://example.com/memory-cache.php?remember=value", handler)
        assert.Equal(t, body, "value")

        body = fetchBody("GET", "http://example.com/memory-cache.php?forget=value", handler)
        assert.Equal(t, body, "nothing")
	}, &testOptions{workerScript: "memory-cache.php", nbWorkers: 1, nbParrallelRequests: 1})
}

func TestReadFromMemoryCacheInParallel(t *testing.T) {
	workerNum := 10
	runTest(t, func(handler func(http.ResponseWriter, *http.Request), _ *httptest.Server, i int) {
		body := fetchBody("GET", "http://example.com/memory-cache.php?remember=value", handler)
		assert.Equal(t, body, "value")

		// Test that all threads have the memory cache
		wg := &sync.WaitGroup{}
		wg.Add(workerNum)
		for i := 0; i < workerNum; i++ {
			go func() {
				body = fetchBody("GET", "http://example.com/memory-cache.php", handler)
				assert.Equal(t, body, "value")
				wg.Done()
			}()
		}
		wg.Wait()
	}, &testOptions{workerScript: "memory-cache.php", nbWorkers: workerNum, nbParrallelRequests: 1})
}