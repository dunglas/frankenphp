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

// AddString adds a string to the pointer list by converting it to a C-char pointer and calling the AddPointer method.
// The string is converted to a C-char pointer using the C.CString function.
// It is recommended to use this method when you need to add a string to the pointer list.
func (p *pointerList) AddString(str *C.char) {
	p.AddPointer(unsafe.Pointer(str))
	//getLogger().Warn("Adding string", zap.Int("i", len(p.Pointers)), zap.String("str", C.GoString(str)), zap.Stack("trace"))
}

// ToCString takes a string and converts it to a C string using C.CString. Then it calls the AddString method of
// pointerList to add the resulting C string as a pointer to the pointer
func (p *pointerList) ToCString(string string) *C.char {
	str := C.CString(string)
	p.AddString(str)
	return str
}

// FreeAll frees all C pointers
func (p *pointerList) FreeAll() {
	for _, ptr := range p.Pointers {
		//getLogger().Warn("About to delete", zap.Int("i", i))
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
