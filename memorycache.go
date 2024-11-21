package frankenphp

// #include "frankenphp.h"
import "C"
import "sync"
import "time"

// TODO:delete oldest cache if maxCacheLength is reached

type memoryCache struct {
	maxCacheLength     int
	currentCacheLength int
	entries            map[string]cacheEntry
	mu                 sync.RWMutex
}

type cacheEntry struct {
	value     string
	expiresAt int64
}

var globalMemoryCache memoryCache

var maxCacheLength int
var cache = make(map[string]string)
var cacheMutex sync.RWMutex

func initMemoryCache(maxCacheLength int) {
	globalMemoryCache = memoryCache{
		maxCacheLength:     maxCacheLength,
		currentCacheLength: 0,
		entries:            make(map[string]cacheEntry),
		mu:                 sync.RWMutex{},
	}
}

func drainMemoryCache() {
	globalMemoryCache.clear()
}

func (m *memoryCache) get(key string) (string, bool) {
	m.mu.RLock()
	entry, ok := m.entries[key]
	m.mu.RUnlock()
	if !ok {
		return "", false
	}
	if entry.expiresAt < time.Now().Unix() {
		m.delete(key)
		return "", false
	}
	return entry.value, ok
}

func (m *memoryCache) set(key string, value string, ttl int64) bool {
	requiredSpace := len(value) + len(key)
	expiresAt := time.Now().Unix() + int64(ttl)
	// if ttl is smaller than 0, set expiresAt to max int64
	if ttl < 0 {
		expiresAt = 9223372036854775807
	}

	if !m.requireSpace(requiredSpace) {
		return false
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// adjust cache length if entry already exists
	if entry, ok := m.entries[key]; ok {
		m.currentCacheLength -= len(entry.value) + len(key)
	}

	// set new entry
	m.entries[key] = cacheEntry{value: value, expiresAt: expiresAt}
	m.currentCacheLength += requiredSpace

	return true
}

func (m *memoryCache) delete(key string) {
	entry, ok := m.entries[key]
	if !ok {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.currentCacheLength -= len(entry.value) + len(key)
	delete(m.entries, key)
}

func (m *memoryCache) clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entries = make(map[string]cacheEntry)
	m.currentCacheLength = 0
}

func (m *memoryCache) requireSpace(requiredSpace int) bool {
	if m.hasSpace(requiredSpace) {
		return true
	}
	if requiredSpace > m.maxCacheLength {
		return false
	}

	// delete entries until enough space is freed
	m.mu.Lock()
	defer m.mu.Unlock()
	for key, entry := range m.entries {
		delete(m.entries, key)
		m.currentCacheLength -= len(entry.value) + len(key)
		if m.currentCacheLength+requiredSpace <= m.maxCacheLength {
			return true
		}
	}
	return false
}

func (m *memoryCache) hasSpace(space int) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.currentCacheLength+space <= m.maxCacheLength
}

//export go_frankenphp_cache_put
func go_frankenphp_cache_put(key *C.char, value *C.char, valueLen C.int, ttl C.zend_long) C.bool {
	goKey := C.GoString(key)
	if value == nil {
		globalMemoryCache.delete(goKey)
		return C.bool(true)
	}
	goValue := C.GoStringN(value, valueLen)
	success := globalMemoryCache.set(goKey, goValue, int64(ttl))
	return C.bool(success)
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
