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
type PointerList struct {
	Pointers []unsafe.Pointer
}

// HandleList A list of pointers that can be freed at a later time
type HandleList struct {
	Handles []cgo.Handle
}

// AddHandle Call when registering a handle for the very first time
func (h *HandleList) AddHandle(handle cgo.Handle) {
	h.Handles = append(h.Handles, handle)
}

// AddPointer Call when creating a request-level C pointer for the very first time
func (p *PointerList) AddPointer(ptr unsafe.Pointer) {
	p.Pointers = append(p.Pointers, ptr)
}

// FreeAll frees all C pointers
func (p *PointerList) FreeAll() {
	for _, ptr := range p.Pointers {
		C.free(ptr)
	}
	p.Pointers = nil // To avoid dangling pointers
}

// FreeAll frees all handles
func (h *HandleList) FreeAll() {
	defer func() {
		if err := recover(); err != nil {
			getLogger().Warn("A handle was already deleted manually, indeterminate state")
		}
	}()
	for _, p := range h.Handles {
		p.Delete()
	}
}

// Pointers Get a new list of pointers
func Pointers() *PointerList {
	return &PointerList{
		Pointers: make([]unsafe.Pointer, 0),
	}
}

// Handles Get a new list of handles
func Handles() *HandleList {
	return &HandleList{
		Handles: make([]cgo.Handle, 0),
	}
}
