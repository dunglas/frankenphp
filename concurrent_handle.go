package frankenphp

import (
	"sync"
	"sync/atomic"
	"unsafe"
)

type Handle uintptr
type HandleKey uintptr

type handlePartition struct {
	value []atomic.Value
	upper uintptr
	lower uintptr
	idx   uintptr
}

// concurrentHandle represents a concurrent handle data structure for storing values associated with handles.
//
// Fields:
// - handles: A map of handle keys to handle partitions
// - nextKey: The next available handle key
// - mu: A read-write mutex for thread-safe access to the handles
// - nll: A nil slot used for deleting values
type concurrentHandle struct {
	handles map[HandleKey]*handlePartition
	nextKey HandleKey
	mu      sync.RWMutex
	nll     *slot
	maxKey  Handle
}

type slot struct {
	value any
}

var (
	ConcurrentHandle = concurrentHandle{
		handles: make(map[HandleKey]*handlePartition),
		nextKey: 0,
		nll:     nil,
		maxKey:  0,
	}
)

// NewConcurrentHandle creates a new concurrent handle with the specified number of handles.
// It acquires a lock, generates a new unique key, and initializes a new handle partition with the given number of handles.
// The new handle partition is then added to the concurrent handle map, and the lock is released.
// Finally, it returns the newly generated handle key.
//
// Go Doc:
// func (h *concurrentHandle) NewConcurrentHandle(nHandles int) HandleKey
//
// Parameters:
// - h: A pointer to the concurrentHandle struct
// - nHandles: The number of handles to initialize in the new handle partition
//
// Returns:
// - HandleKey: The newly generated handle key
//
// Example Usage:
// requestHandles = ConcurrentHandle.NewConcurrentHandle(opt.numThreads * 4)
func (h *concurrentHandle) NewConcurrentHandle(nHandles int) HandleKey {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.maxKey == 0 {
		// determine half the bits of the key
		h.maxKey = Handle(unsafe.Sizeof(Handle(0)) * 4)
	}

	key := atomic.AddUintptr((*uintptr)(&h.nextKey), 1)

	if key > uintptr(1<<h.maxKey) {
		panic("concurrentHandle: maximum number of concurrent handles reached")
	}

	h.handles[HandleKey(key)] = &handlePartition{
		value: make([]atomic.Value, nHandles+1),
		// pack the key into the index, so we can look up the key from the handle
		idx:   uintptr(key) << h.maxKey,
		lower: uintptr(key) << h.maxKey,
		upper: uintptr(key+1) << h.maxKey,
	}

	return HandleKey(key)
}

// NewHandle creates a new handle with the specified value.
// It acquires a read lock, retrieves the handle partition associated with the given handle key,
// creates a new slot with the specified value, and loops to find the next available slot to store the new value.
// Once a slot is found, it uses atomic operations to swap or set the value in the slot.
// Finally, it returns the handle corresponding to the slot where the value was stored.
func (k *HandleKey) NewHandle(v any) Handle {
	ConcurrentHandle.mu.RLock()
	defer ConcurrentHandle.mu.RUnlock()

	h := ConcurrentHandle.handles[*k]
	s := &slot{value: v}

	for {
		// attempt to get the next handle slot
		next := atomic.AddUintptr(&h.idx, 1)
		sloti := next % uintptr(len(h.value))

		// 0 should not be an option, because it is "falsy"
		if next == h.lower {
			continue
		}

		// we have reached the end of the partition, try to wrap around
		if next > h.upper {
			if atomic.CompareAndSwapUintptr(&h.idx, next, h.lower+1) {
				next = h.lower + 1
			} else {
				continue
			}
		}

		// set the value in the slot
		if h.value[sloti].CompareAndSwap(ConcurrentHandle.nll, s) {
			return Handle(next)
		} else if h.value[sloti].CompareAndSwap(nil, s) {
			return Handle(next)
		}
	}
}

// Value returns the value associated with the given Handle in the handle partition associated with the given HandleKey.
// It acquires a read lock, retrieves the value from the handle partition using the provided Handle,
// and returns the value if it is not nil.
//
// Parameters:
// - k: A pointer to the HandleKey struct
// - h: The Handle to retrieve the value for
//
// Returns:
// - any: The value associated with the Handle, or nil if the value is nil
func (k *HandleKey) Value(h Handle) any {
	ConcurrentHandle.mu.RLock()
	defer ConcurrentHandle.mu.RUnlock()

	sloti := len(ConcurrentHandle.handles[*k].value)
	val := ConcurrentHandle.handles[*k].value[h%Handle(sloti)].Load()
	if val == nil {
		return nil
	}
	return val.(*slot).value
}

func (h Handle) Value() any {
	k := HandleKey(uintptr(h) >> ConcurrentHandle.maxKey)
	return k.Value(h)
}

// Delete deletes the value stored in the specified handle of the handle partition associated with the given HandleKey.
// It acquires a read lock, sets the value at the specified handle index to nil using atomic operations, and releases the lock.
// Parameters:
// - h: The handle key associated with the handle partition containing the value to delete.
// - h: The handle index of the value to delete.
func (k *HandleKey) Delete(h Handle) {
	ConcurrentHandle.mu.RLock()
	defer ConcurrentHandle.mu.RUnlock()

	// we have already deleted this handle
	if *k == HandleKey(0) {
		return
	}
	sloti := len(ConcurrentHandle.handles[*k].value)
	ConcurrentHandle.handles[*k].value[h%Handle(sloti)].Store(ConcurrentHandle.nll)
}

func (h Handle) Delete() {
	k := HandleKey(uintptr(h) >> ConcurrentHandle.maxKey)
	k.Delete(h)
}
