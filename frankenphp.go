package frankenphp

// #cgo CFLAGS: -Wall -Wno-unused-variable
// #cgo CFLAGS: -I/usr/local/include/php -I/usr/local/include/php/Zend -I/usr/local/include/php/TSRM -I/usr/local/include/php/main
// #cgo LDFLAGS: -L/usr/local/lib -L/opt/homebrew/opt/libiconv/lib -lphp -lxml2 -liconv -lresolv -lsqlite3
// #include <stdlib.h>
// #include <stdint.h>
// #include "php_variables.h"
// #include "frankenphp.h"
import "C"
import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"runtime"
	"runtime/cgo"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"
)

var started int32
var shutdown chan interface{}

type ContextKey string

const FrankenPHPContextKey ContextKey = "frankenphp"

// FrankenPHP executes PHP scripts.
type FrankenPHPContext struct {
	// The root directory of the PHP application.
	DocumentRoot string

	// The path in the URL will be split into two, with the first piece ending
	// with the value of SplitPath. The first piece will be assumed as the
	// actual resource (CGI script) name, and the second piece will be set to
	// PATH_INFO for the CGI script to use.
	//
	// Future enhancements should be careful to avoid CVE-2019-11043,
	// which can be mitigated with use of a try_files-like behavior
	// that 404s if the fastcgi path info is not found.
	SplitPath []string

	// Path declared as root directory will be resolved to its absolute value
	// after the evaluation of any symbolic links.
	// Due to the nature of PHP opcache, root directory path is cached: when
	// using a symlinked directory as root this could generate errors when
	// symlink is changed without php-fpm being restarted; enabling this
	// directive will set $_SERVER['DOCUMENT_ROOT'] to the real directory path.
	ResolveRootSymlink bool

	// CGI-like environment variables that will be available in $_SERVER.
	// This map is populated automatically, exisiting key are never replaced.
	Env map[string]string

	responseWriter http.ResponseWriter
	done           chan interface{}
}

func NewRequestWithContext(r *http.Request, documentRoot string) *http.Request {
	ctx := context.WithValue(r.Context(), FrankenPHPContextKey, &FrankenPHPContext{
		DocumentRoot: documentRoot,
		SplitPath:    []string{".php"},
		Env:          make(map[string]string),
	})

	return r.WithContext(ctx)
}

// Startup starts the PHP engine.
// Startup and Shutdown must be called in the same goroutine (ideally in the main function).
func Startup() error {
	if atomic.LoadInt32(&started) > 0 {
		return nil
	}
	atomic.StoreInt32(&started, 1)

	runtime.LockOSThread()

	if C.frankenphp_init() < 0 {
		return fmt.Errorf(`ZTS is not enabled, recompile PHP using the "--enable-zts" configuration option`)
	}

	shutdown = make(chan interface{})

	return nil
}

// Shutdown stops the PHP engine.
// Shutdown and Startup must be called in the same goroutine (ideally in the main function).
func Shutdown() {
	if atomic.LoadInt32(&started) < 1 {
		return
	}
	atomic.StoreInt32(&started, 0)

	close(shutdown)

	var waitForWorkers bool
	workers.Range(func(k, v interface{}) bool {
		waitForWorkers = true
		workers.Delete(k)

		return true
	})

	if waitForWorkers {
		// Give some time to workers to finish gracefuly
		time.Sleep(1 * time.Second)
	}

	C.frankenphp_shutdown()
}

func updateServerContext(request *http.Request) error {
	authPassword, err := populateEnv(request)
	if err != nil {
		return err
	}

	fc := request.Context().Value(FrankenPHPContextKey).(*FrankenPHPContext)

	var cAuthUser, cAuthPassword *C.char
	if authPassword != "" {
		cAuthPassword = C.CString(authPassword)
	}

	if authUser := fc.Env["REMOTE_USER"]; authUser != "" {
		cAuthUser = C.CString(authUser)
	}

	rh := cgo.NewHandle(request)

	cMethod := C.CString(request.Method)
	cQueryString := C.CString(request.URL.RawQuery)
	contentLengthStr := request.Header.Get("Content-Length")
	contentLength := 0
	if contentLengthStr != "" {
		contentLength, _ = strconv.Atoi(contentLengthStr)
	}

	contentType := request.Header.Get("Content-Type")
	var cContentType *C.char
	if contentType != "" {
		cContentType = C.CString(contentType)
	}

	var cPathTranslated *C.char
	if pathTranslated := fc.Env["PATH_TRANSLATED"]; pathTranslated != "" {
		cPathTranslated = C.CString(pathTranslated)
	}

	cRequestUri := C.CString(request.URL.RequestURI())

	C.frankenphp_update_server_context(
		C.uintptr_t(rh),

		cMethod,
		cQueryString,
		C.zend_long(contentLength),
		cPathTranslated,
		cRequestUri,
		cContentType,
		cAuthUser,
		cAuthPassword,
		C.int(request.ProtoMajor*1000+request.ProtoMinor),
	)

	return nil
}

func ExecuteScript(responseWriter http.ResponseWriter, request *http.Request) error {
	if atomic.LoadInt32(&started) < 1 {
		panic("FrankenPHP isn't started, call frankenphp.Startup()")
	}

	runtime.LockOSThread()
	// todo: check if it's ok or not to call runtime.UnlockOSThread() to reuse this thread

	if C.frankenphp_create_server_context(0) < 0 {
		return fmt.Errorf("error during request context creation")
	}

	if err := updateServerContext(request); err != nil {
		return err
	}

	if C.frankenphp_request_startup() < 0 {
		return fmt.Errorf("error during PHP request startup")
	}

	fc := request.Context().Value(FrankenPHPContextKey).(*FrankenPHPContext)
	fc.responseWriter = responseWriter

	cFileName := C.CString(fc.Env["SCRIPT_FILENAME"])
	defer C.free(unsafe.Pointer(cFileName))

	if C.frankenphp_execute_script(cFileName) < 0 {
		return fmt.Errorf("error during PHP script execution")
	}
	C.frankenphp_request_shutdown()

	return nil
}

func newWorker(fileName string, pool sync.Pool) *worker {
	w := &worker{
		pool,
		make(chan *http.Request),
	}
	wh := cgo.NewHandle(w)

	go func() {
		runtime.LockOSThread()

		if C.frankenphp_create_server_context(C.uintptr_t(wh)) < 0 {
			panic(fmt.Errorf("error during request context creation"))
		}

		if C.frankenphp_request_startup() < 0 {
			panic("error during PHP request startup")
		}

		// todo: maybe could we allocate this only once
		cFileName := C.CString(fileName)
		defer C.free(unsafe.Pointer(cFileName))

		if C.frankenphp_execute_script(cFileName) < 0 {
			panic("error during PHP script execution")
		}
		C.frankenphp_request_shutdown()
	}()

	return w
}

//export go_frankenphp_handle_request
func go_frankenphp_handle_request(wh C.uintptr_t) bool {
	w := cgo.Handle(wh).Value().(*worker)

	select {
	case <-shutdown:
		// channel closed, server is shutting down
		return false

	case r := <-w.in:
		fc := r.Context().Value(FrankenPHPContextKey).(*FrankenPHPContext)
		if fc == nil || fc.responseWriter == nil {
			panic("not in worker mode")
		}

		fc.responseWriter.Write([]byte(fmt.Sprintf("Hello from Go: %#v", r)))
		close(fc.done)

		w.pool.Put(w)
	}

	return true
}

//export go_ub_write
func go_ub_write(rh C.uintptr_t, cString *C.char, length C.int) C.size_t {
	r := cgo.Handle(rh).Value().(*http.Request)
	fc := r.Context().Value(FrankenPHPContextKey).(*FrankenPHPContext)

	i, _ := fc.responseWriter.Write([]byte(C.GoStringN(cString, length)))

	return C.size_t(i)
}

//export go_register_variables
func go_register_variables(rh C.uintptr_t, trackVarsArray *C.zval) {
	r := cgo.Handle(rh).Value().(*http.Request)
	for k, v := range r.Context().Value(FrankenPHPContextKey).(*FrankenPHPContext).Env {
		ck := C.CString(k)
		cv := C.CString(v)
		C.php_register_variable_safe(ck, cv, C.size_t(len(v)), trackVarsArray)

		C.free(unsafe.Pointer(ck))
		C.free(unsafe.Pointer(cv))
	}
}

//export go_add_header
func go_add_header(rh C.uintptr_t, cString *C.char, length C.int) {
	r := cgo.Handle(rh).Value().(*http.Request)
	fc := r.Context().Value(FrankenPHPContextKey).(*FrankenPHPContext)

	parts := strings.SplitN(C.GoStringN(cString, length), ": ", 2)
	if len(parts) != 2 {
		log.Printf(`invalid header "%s"`+"\n", parts[0])

		return
	}

	fc.responseWriter.Header().Add(parts[0], parts[1])
}

//export go_write_header
func go_write_header(rh C.uintptr_t, status C.int) {
	r := cgo.Handle(rh).Value().(*http.Request)
	fc := r.Context().Value(FrankenPHPContextKey).(*FrankenPHPContext)

	fc.responseWriter.WriteHeader(int(status))
}

//export go_read_post
func go_read_post(rh C.uintptr_t, cBuf *C.char, countBytes C.size_t) C.size_t {
	r := cgo.Handle(rh).Value().(*http.Request)

	p := make([]byte, int(countBytes))
	readBytes, err := r.Body.Read(p)
	if err != nil && err != io.EOF {
		panic(err)
	}

	if readBytes != 0 {
		//todo: memory leak?
		C.memcpy(unsafe.Pointer(cBuf), unsafe.Pointer(&p[0]), C.size_t(readBytes))
	}

	return C.size_t(readBytes)
}

//export go_read_cookies
func go_read_cookies(rh C.uintptr_t) *C.char {
	r := cgo.Handle(rh).Value().(*http.Request)

	cookies := r.Cookies()
	if len(cookies) == 0 {
		return nil
	}

	cookieString := make([]string, len(cookies))
	for _, cookie := range r.Cookies() {
		cookieString = append(cookieString, cookie.String())
	}

	cCookie := C.CString(strings.Join(cookieString, "; "))
	// freed in frankenphp_request_shutdown()

	return cCookie
}

//export go_clean_server_context
func go_clean_server_context(rh C.uintptr_t) {
	cgo.Handle(rh).Delete()
}
