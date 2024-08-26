// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found at https://github.com/golang/go/blob/master/LICENSE.

package frankenphp

import (
	"sync"
	"sync/atomic"
)

// handle is based on the CL here: https://go-review.googlesource.com/c/go/+/600875
type handle uintptr

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

func newHandle(v any) handle {
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
			return handle(h)
		} else if handles[h-1].CompareAndSwap(nil, s) {
			growLock.RUnlock()
			return handle(h)
		} else {
			h++
		}
		growLock.RUnlock()
	}
}

func (h handle) Value() any {
	growLock.RLock()
	defer growLock.RUnlock()
	if h > handle(len(handles)) {
		panic("runtime/cgo: misuse of an invalid handle")
	}

	v := handles[h-1].Load()
	if v == nil || v == nilSlot {
		panic("runtime/cgo: misuse of an released handle")
	}

	return v.value
}

func (h handle) Delete() {
	growLock.RLock()
	defer growLock.RUnlock()
	if h == 0 {
		panic("runtime/cgo: misuse of an zero handle")
	}

	if h > handle(len(handles)) {
		panic("runtime/cgo: misuse of an invalid handle")
	}

	if v := handles[h-1].Swap(nilSlot); v == nil || v == nilSlot {
		panic("runtime/cgo: misuse of an released handle")
	}
	//nolint:staticcheck
	releasedIdx.Put(uint64(h))
}
