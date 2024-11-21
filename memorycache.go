package frankenphp

// #include "frankenphp.h"
import "C"
import "sync"

// TODO:delete oldest cache if maxCacheLength is reached

type memoryCache struct {
	maxCacheLength int
	currentCacheLength int
	cache map[string]string
	mu sync.RWMutex
	keys []string
}

var globalMemoryCache memoryCache

var maxCacheLength int
var cache = make(map[string]string)
var cacheMutex sync.RWMutex

func initMemoryCache(maxCacheLength int){
	globalMemoryCache = memoryCache {
		maxCacheLength: maxCacheLength,
		currentCacheLength: 0,
		cache: make(map[string]string),
		mu: sync.RWMutex{},
		keys: make([]string, 0),
	}
}

func drainMemoryCache() {
	globalMemoryCache.clear()
}

func (m *memoryCache) get(key string) (string, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	value, ok := m.cache[key]
	return value, ok
}

func (m *memoryCache) set(key string, value string) {
	m.mu.RLock()
    defer m.mu.RUnlock()
	m.cache[key] = value
	m.currentCacheLength += len(value)
}

func (m *memoryCache) delete(key string) {
	m.mu.RLock()
    defer m.mu.RUnlock()
    value, ok := m.cache[key]
    if !ok {
    	return
	}
	m.currentCacheLength -= len(value)
	delete(m.cache, key)
}

func (m *memoryCache) clear() {
	m.mu.RLock()
    defer m.mu.RUnlock()
	m.cache = make(map[string]string)
	m.currentCacheLength = 0
}

//export go_frankenphp_cache_put
func go_frankenphp_cache_put(key *C.char, value *C.char, valueLen C.int) C.bool{
    goKey := C.GoString(key)
    if value == nil {
        globalMemoryCache.delete(goKey)
    }
    goValue := C.GoStringN(value, valueLen)
    globalMemoryCache.set(goKey, goValue)
    return C.bool(true)
}

//export go_frankenphp_cache_get
func go_frankenphp_cache_get(key *C.char) (*C.char, C.size_t) {
    goKey := C.GoString(key)
    goValue, ok := globalMemoryCache.get(goKey)

    if !ok {
		return nil, 0
	}

    // note: PHP handles freeing the memory of the returned string
    return C.CString(goValue), C.size_t(len(goValue))
}

//export go_frankenphp_cache_forget
func go_frankenphp_cache_forget(key *C.char) {
    goKey := C.GoString(key)
    globalMemoryCache.delete(goKey)
}