package frankenphp

// #include "frankenphp.h"
import "C"
import (
	"os"
	"strings"
	"unsafe"
)

// TODO: initialize as zend_string, remove pinning
func getEnvAsMap() map[string]string {
	env := os.Environ()
	envMap := make(map[string]string, len(env))

	for _, envVar := range env {
		key, val, _ := strings.Cut(envVar, "=")
		envMap[key] = val
	}

	return envMap
}

//export go_putenv
func go_putenv(threadIndex C.uintptr_t, str *C.char, length C.int) C.bool {
	thread := phpThreads[threadIndex]
	envString := C.GoStringN(str, length)
	thread.requireSandboxedEnv()

	// Check if '=' is present in the string
	if key, val, found := strings.Cut(envString, "="); found {
		thread.sandboxedEnv[key] = val
		return os.Setenv(key, val) == nil
	}

	// No '=', unset the environment variable
	delete(thread.sandboxedEnv, envString)
	return os.Unsetenv(envString) == nil
}

//export go_getfullenv
func go_getfullenv(threadIndex C.uintptr_t) (*C.go_string, C.size_t) {
	thread := phpThreads[threadIndex]
	env := thread.getSandboxedEnv()
	goStrings := make([]C.go_string, 0, len(env))

	for key, val := range env {
		goStrings = append(goStrings, C.go_string{C.size_t(len(key)), thread.pinString(key)})
		goStrings = append(goStrings, C.go_string{C.size_t(len(val)), thread.pinString(val)})
	}

	value := unsafe.SliceData(goStrings)
	thread.Pin(value)

	return value, C.size_t(len(env))
}

//export go_getenv
func go_getenv(threadIndex C.uintptr_t, name *C.go_string) (C.bool, *C.go_string) {
	thread := phpThreads[threadIndex]

	// Create a byte slice from C string with a specified length
	envName := C.GoStringN(name.data, C.int(name.len))

	// Get the environment variable value
	envValue, exists := thread.getSandboxedEnv()[envName]
	if !exists {
		// Environment variable does not exist
		return false, nil // Return 0 to indicate failure
	}

	// Convert Go string to C string
	value := &C.go_string{C.size_t(len(envValue)), thread.pinString(envValue)}
	thread.Pin(value)

	return true, value // Return 1 to indicate success
}

//export go_sapi_getenv
func go_sapi_getenv(threadIndex C.uintptr_t, name *C.go_string) *C.char {
	thread := phpThreads[threadIndex]
	envName := C.GoStringN(name.data, C.int(name.len))

	envValue, exists := thread.getSandboxedEnv()[envName]
	if !exists {
		return nil
	}

	return phpThreads[threadIndex].pinCString(envValue)
}
