package frankenphp

//#include <zend.h>
import "C"
import "unsafe"

// EXPERIMENTAL: GoString converts a zend_string to a Go string without copy.
func GoString(s unsafe.Pointer) string {
	if s == nil {
		return ""
	}

	zendStr := (*C.zend_string)(s)

	return C.GoStringN((*C.char)(unsafe.Pointer(&zendStr.val)), C.int(zendStr.len))
}
