package frankenphp

// #include "frankenphp.h"
import "C"
import "sync"

var maxCacheLength int
var cache = make(map[string]string)
var cacheMutex sync.RWMutex

func initMemoryCache(maxCacheLength int){
	maxCacheLength = maxCacheLength
}

func drainMemoryCache() {
	memoryCacheClear()
}

func memoryCacheGet(key string) (string, bool) {
	cacheMutex.RLock()
	defer cacheMutex.RUnlock()
	value, ok := cache[key]
	return value, ok
}

func memoryCacheSet(key string, value string) {
	cacheMutex.Lock()
	defer cacheMutex.Unlock()
	cache[key] = value
}

func memoryCacheDelete(key string) {
	cacheMutex.Lock()
	defer cacheMutex.Unlock()
	delete(cache, key)
}

func memoryCacheClear() {
	cacheMutex.Lock()
	defer cacheMutex.Unlock()
	cache = make(map[string]string)
}

//export go_frankenphp_cache_put
func go_frankenphp_cache_put(key *C.char, value *C.char) C.bool{
    goKey := C.GoString(key)
    if value == nil {
        memoryCacheDelete(goKey)
    }
    goValue := C.GoString(value)
    memoryCacheSet(goKey, goValue)
    return C.bool(true)
}

//export go_frankenphp_cache_get
func go_frankenphp_cache_get(key *C.char) *C.char {
    goKey := C.GoString(key)
    goValue, ok := memoryCacheGet(goKey)
    if !ok {
		return nil
	}
    return C.CString(goValue)
}

// TODO: forget and flush
//export go_frankenphp_cache_forget
func go_frankenphp_cache_forget(key *C.char) {
    goKey := C.GoString(key)
    memoryCacheDelete(goKey)
}