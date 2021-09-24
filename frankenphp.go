package frankenphp

// #cgo CFLAGS: -Wall -Wno-unused-variable -g
// #cgo CFLAGS: -I/usr/local/include/php -I/usr/local/include/php/Zend -I/usr/local/include/php/TSRM -I/usr/local/include/php/main
// #cgo LDFLAGS: -L/usr/local/lib -lphp
// #include <stdlib.h>
// #include "frankenphp.h"
import "C"
import (
	"fmt"
	"unsafe"
)

func Startup() error {
	if C.frankenphp_init() < 0 {
		return fmt.Errorf(`ZTS is not enabled, recompile PHP using the "--enable-zts" configuration option`)
	}

	return nil
}

func Shutdown() {
	C.frankenphp_shutdown()
}

func ExecuteScript(fileName string) error {
	if C.frankenphp_request_startup() < 0 {
		return fmt.Errorf("error during PHP request startup")
	}

	cFileName := C.CString(fileName)
	defer C.free(unsafe.Pointer(cFileName))
	C.frankenphp_execute_script(cFileName)

	C.frankenphp_request_shutdown()

	return nil
}
