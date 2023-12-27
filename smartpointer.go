package frankenphp

// #include <stdlib.h>
import "C"
import (
	"runtime/cgo"
	"unsafe"
)

/*
FrankenPHP is fairly complex because it shuffles handles/requests/contexts
between C and Go. This simplifies the lifecycle management of per-request
structures by allowing us to hold references until the end of the request
and ensure they are always cleaned up.
*/

// PointerList A list of pointers that can be freed at a later time
type pointerList struct {
	Pointers []unsafe.Pointer
}

// HandleList A list of pointers that can be freed at a later time
type handleList struct {
	Handles []cgo.Handle
}

// AddHandle Call when registering a handle for the very first time
func (h *handleList) AddHandle(handle cgo.Handle) {
	h.Handles = append(h.Handles, handle)
}

// AddPointer Call when creating a request-level C pointer for the very first time
func (p *pointerList) AddPointer(ptr unsafe.Pointer) {
	p.Pointers = append(p.Pointers, ptr)
}

// FreeAll frees all C pointers
func (p *pointerList) FreeAll() {
	for _, ptr := range p.Pointers {
		C.free(ptr)
	}
	p.Pointers = nil // To avoid dangling pointers
}

// FreeAll frees all handles
func (h *handleList) FreeAll() {
	for _, p := range h.Handles {
		p.Delete()
	}
}

// Pointers Get a new list of pointers
func Pointers() *pointerList {
	return &pointerList{
		Pointers: make([]unsafe.Pointer, 0),
	}
}

// Handles Get a new list of handles
func Handles() *handleList {
	return &handleList{
		Handles: make([]cgo.Handle, 0, 8),
	}
}
