package frankenphp

//#include <Zend/zend_string.h>
//
//zend_string *hello_string() {
//	return zend_string_init("Hello", 5, 1);
//}
import "C"
import (
	"github.com/stretchr/testify/assert"
	"testing"
	"unsafe"
)

func testGoString(t *testing.T) {
	assert.Equal(t, "", GoString(nil))
	assert.Equal(t, "Hello", GoString(unsafe.Pointer(C.hello_string())))
}
