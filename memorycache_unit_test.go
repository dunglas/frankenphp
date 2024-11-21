package frankenphp

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOverwriteCacheEntry(t *testing.T) {
	initMemoryCache(100)
	defer drainMemoryCache()

	success1 := globalMemoryCache.set("key", "value", -1)
	success2 := globalMemoryCache.set("key", "value2", -1)

	value, ok := globalMemoryCache.get("key")

	assert.True(t, ok)
	assert.True(t, success1)
	assert.True(t, success2)
	assert.Equal(t, "value2", value)
	assert.Equal(t, len("key"+"value2"), globalMemoryCache.currentCacheLength)
}

func TestExpireCacheEntry(t *testing.T) {
	initMemoryCache(100)
	defer drainMemoryCache()

	success := globalMemoryCache.set("key", "value", -1)
	globalMemoryCache.entries["key"] = cacheEntry{value: "value", expiresAt: 0}

	_, ok := globalMemoryCache.get("key")

	assert.True(t, success)
	assert.False(t, ok)
}

func TestDeleteCacheEntry(t *testing.T) {
	initMemoryCache(100)
	defer drainMemoryCache()

	success := globalMemoryCache.set("key", "value", -1)
	globalMemoryCache.delete("key")

	_, ok := globalMemoryCache.get("key")

	assert.True(t, success)
	assert.False(t, ok)
	assert.Equal(t, 0, globalMemoryCache.currentCacheLength)
}

func TestDoNotSaveEntryThatIsTooBig(t *testing.T) {
	initMemoryCache(1)
	defer drainMemoryCache()

	success := globalMemoryCache.set("key", "value", -1)

	_, ok := globalMemoryCache.get("key")

	assert.False(t, success)
	assert.False(t, ok)
}

func TestExpirePreviousEntryIfCacheIsTooFull(t *testing.T) {
	initMemoryCache(len("key1value1"))
	defer drainMemoryCache()

	success1 := globalMemoryCache.set("key1", "value1", -1)
	value1, _ := globalMemoryCache.get("key1")

	assert.Equal(t, len("key1value1"), globalMemoryCache.currentCacheLength)
	assert.True(t, success1)
	assert.Equal(t, "value1", value1)

	success2 := globalMemoryCache.set("k2", "value2", -1)
	value2, _ := globalMemoryCache.get("k2")
	_, ok := globalMemoryCache.get("key1")

	assert.True(t, success2)
	assert.False(t, ok)
	assert.Equal(t, "value2", value2)
	assert.Equal(t, len("k2value2"), globalMemoryCache.currentCacheLength)
}
