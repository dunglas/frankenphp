package frankenphp

//#include <zend.h>
import "C"
import "unsafe"

// GoString converts a zend_string to a Go string without copy.
func GoString(zendStr *C.zend_string) string {
	if zendStr == nil {
		return ""
	}

	return C.GoStringN((*C.char)(unsafe.Pointer(&zendStr.val)), C.int(zendStr.len))
}
