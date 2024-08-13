// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found at https://github.com/golang/go/blob/master/LICENSE.

package frankenphp

import (
	"sync"
	"sync/atomic"
)

// Handle provides a way to pass values that contain Go pointers
// (pointers to memory allocated by Go) between Go and C without
// breaking the cgo pointer passing rules. A Handle is an integer
// value that can represent any Go value. A Handle can be passed
// through C and back to Go, and Go code can use the Handle to
// retrieve the original Go value.
//
// The underlying type of Handle is guaranteed to fit in an integer type
// that is large enough to hold the bit pattern of any pointer. The zero
// value of a Handle is not valid, and thus is safe to use as a sentinel
// in C APIs.
//
// For instance, on the Go side:
//
//	package main
//
//	/*
//	#include <stdint.h> // for uintptr_t
//
//	extern void MyGoPrint(uintptr_t handle);
//	void myprint(uintptr_t handle);
//	*/
//	import "C"
//	import "runtime/cgo"
//
//	//export MyGoPrint
//	func MyGoPrint(handle C.uintptr_t) {
//		h := cgo.Handle(handle)
//		val := h.Value().(string)
//		println(val)
//		h.Delete()
//	}
//
//	func main() {
//		val := "hello Go"
//		C.myprint(C.uintptr_t(cgo.NewHandle(val)))
//		// Output: hello Go
//	}
//
// and on the C side:
//
//	#include <stdint.h> // for uintptr_t
//
//	// A Go function
//	extern void MyGoPrint(uintptr_t handle);
//
//	// A C function
//	void myprint(uintptr_t handle) {
//	    MyGoPrint(handle);
//	}
//
// Some C functions accept a void* argument that points to an arbitrary
// data value supplied by the caller. It is not safe to coerce a [cgo.Handle]
// (an integer) to a Go [unsafe.Pointer], but instead we can pass the address
// of the cgo.Handle to the void* parameter, as in this variant of the
// previous example:
//
//	package main
//
//	/*
//	extern void MyGoPrint(void *context);
//	static inline void myprint(void *context) {
//	    MyGoPrint(context);
//	}
//	*/
//	import "C"
//	import (
//		"runtime/cgo"
//		"unsafe"
//	)
//
//	//export MyGoPrint
//	func MyGoPrint(context unsafe.Pointer) {
//		h := *(*cgo.Handle)(context)
//		val := h.Value().(string)
//		println(val)
//		h.Delete()
//	}
//
//	func main() {
//		val := "hello Go"
//		h := cgo.NewHandle(val)
//		C.myprint(unsafe.Pointer(&h))
//		// Output: hello Go
//	}
type Handle uintptr

// slot represents a slot in the handles slice for concurrent access.
type slot struct {
	value any
}

var (
	handles     []*atomic.Pointer[slot]
	releasedIdx sync.Pool
	nilSlot     = &slot{}
	growLock    sync.RWMutex
	handLen     atomic.Uint64
)

func init() {
	handles = make([]*atomic.Pointer[slot], 0)
	handLen = atomic.Uint64{}
	releasedIdx.New = func() interface{} {
		return nil
	}
	growLock = sync.RWMutex{}
}

// NewHandle returns a handle for a given value.
//
// The handle is valid until the program calls Delete on it. The handle
// uses resources, and this package assumes that C code may hold on to
// the handle, so a program must explicitly call Delete when the handle
// is no longer needed.
//
// The intended use is to pass the returned handle to C code, which
// passes it back to Go, which calls Value.
func NewHandle(v any) Handle {
	var h uint64 = 1
	s := &slot{value: v}
	for {
		if released := releasedIdx.Get(); released != nil {
			h = released.(uint64)
		}

		if h > handLen.Load() {
			growLock.Lock()
			handles = append(handles, &atomic.Pointer[slot]{})
			handLen.Store(uint64(len(handles)))
			growLock.Unlock()
		}

		growLock.RLock()
		if handles[h-1].CompareAndSwap(nilSlot, s) {
			growLock.RUnlock()
			return Handle(h)
		} else if handles[h-1].CompareAndSwap(nil, s) {
			growLock.RUnlock()
			return Handle(h)
		} else {
			h++
		}
		growLock.RUnlock()
	}
}

// Value returns the associated Go value for a valid handle.
//
// The method panics if the handle is invalid.
func (h Handle) Value() any {
	growLock.RLock()
	defer growLock.RUnlock()
	if h > Handle(len(handles)) {
		panic("runtime/cgo: misuse of an invalid Handle")
	}

	v := handles[h-1].Load()
	if v == nil || v == nilSlot {
		panic("runtime/cgo: misuse of an released Handle")
	}

	return v.value
}

// Delete invalidates a handle. This method should only be called once
// the program no longer needs to pass the handle to C and the C code
// no longer has a copy of the handle value.
//
// The method panics if the handle is invalid.
func (h Handle) Delete() {
	growLock.RLock()
	defer growLock.RUnlock()
	if h == 0 {
		panic("runtime/cgo: misuse of an zero Handle")
	}

	if h > Handle(len(handles)) {
		panic("runtime/cgo: misuse of an invalid Handle")
	}

	if v := handles[h-1].Swap(nilSlot); v == nil || v == nilSlot {
		panic("runtime/cgo: misuse of an released Handle")
	}
	//nolint:staticcheck
	releasedIdx.Put(uint64(h))
}
