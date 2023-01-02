// Package frankenphp embeds PHP in Go projects and provides a SAPI for net/http.
//
// This is the core of the [FrankenPHP app server], and can be used in any Go program.
//
// [FrankenPHP app server]: https://frankenphp.dev
package frankenphp

//go:generate rm -Rf C-Thread-Pool/
//go:generate git clone --branch=fix/SA_ONSTACK --depth=1 git@github.com:dunglas/C-Thread-Pool.git
//go:generate rm -Rf C-Thread-Pool/.git C-Thread-Pool/.circleci C-Thread-Pool/docs C-Thread-Pool/tests

// #cgo CFLAGS: -Wall -Werror
// #cgo CFLAGS: -I/usr/local/include/php -I/usr/local/include/php/Zend -I/usr/local/include/php/TSRM -I/usr/local/include/php/main
// #cgo linux CFLAGS: -D_GNU_SOURCE
// #cgo LDFLAGS: -L/usr/local/lib -L/opt/homebrew/opt/libiconv/lib -L/usr/lib -lphp -lxml2 -lresolv -lsqlite3 -ldl -lm -lutil
// #cgo darwin LDFLAGS: -liconv
// #include <stdlib.h>
// #include <stdint.h>
// #include <php_variables.h>
// #include "frankenphp.h"
import "C"
import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"runtime/cgo"
	"strconv"
	"strings"
	"sync"
	"unsafe"

	"go.uber.org/zap"
	// debug on Linux
	//_ "github.com/ianlancetaylor/cgosymbolizer"
)

type key int

var contextKey key

var (
	InvalidRequestError         = errors.New("not a FrankenPHP request")
	AlreaydStartedError         = errors.New("FrankenPHP is already started")
	InvalidPHPVersionError      = errors.New("FrankenPHP is only compatible with PHP 8.2+")
	ZendSignalsError            = errors.New("Zend Signals are enabled, recompile PHP with --disable-zend-signals")
	NotEnoughThreads            = errors.New("the number of threads must be superior to the number of workers")
	MainThreadCreationError     = errors.New("error creating the main thread")
	RequestContextCreationError = errors.New("error during request context creation")
	RequestStartupError         = errors.New("error during PHP request startup")
	ScriptExecutionError        = errors.New("error during PHP script execution")

	requestChan chan *http.Request
	shutdownWG  sync.WaitGroup

	loggerMu sync.RWMutex
	logger   *zap.Logger
)

type syslogLevel int

const (
	emerg   syslogLevel = iota // system is unusable
	alert                      // action must be taken immediately
	crit                       // critical conditions
	err                        // error conditions
	warning                    // warning conditions
	notice                     // normal but significant condition
	info                       // informational
	debug                      // debug-level messages
)

func (l syslogLevel) String() string {
	switch l {
	case emerg:
		return "emerg"
	case alert:
		return "alert"
	case crit:
		return "crit"
	case err:
		return "err"
	case warning:
		return "warning"
	case notice:
		return "notice"
	case debug:
		return "debug"
	default:
		return "info"
	}
}

// FrankenPHPContext provides contextual information about the Request to handle.
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

	// The logger associated with the current request
	Logger *zap.Logger

	populated    bool
	authPassword string

	// Whether the request is already closed by us
	closed sync.Once

	responseWriter http.ResponseWriter
	done           chan interface{}
}

func clientHasClosed(r *http.Request) bool {
	select {
	case <-r.Context().Done():
		return true
	default:
		return false
	}
}

// NewRequestWithContext creates a new FrankenPHP request context.
func NewRequestWithContext(r *http.Request, documentRoot string, l *zap.Logger) *http.Request {
	if l == nil {
		l = getLogger()
	}

	ctx := context.WithValue(r.Context(), contextKey, &FrankenPHPContext{
		DocumentRoot: documentRoot,
		SplitPath:    []string{".php"},
		Env:          make(map[string]string),
		Logger:       l,
	})

	return r.WithContext(ctx)
}

// FromContext extracts the FrankenPHPContext from a context.
func FromContext(ctx context.Context) (fctx *FrankenPHPContext, ok bool) {
	fctx, ok = ctx.Value(contextKey).(*FrankenPHPContext)
	return
}

type PHPVersion struct {
	MajorVersion   int
	MinorVersion   int
	ReleaseVersion int
	ExtraVersion   string
	Version        string
	VersionID      int
}

type PHPConfig struct {
	Version     PHPVersion
	ZTS         bool
	ZendSignals bool
	ZendTimer   bool
}

// Version returns infos about the PHP version.
func Version() PHPVersion {
	cVersion := C.frankenphp_get_version()

	return PHPVersion{
		int(cVersion.major_version),
		int(cVersion.minor_version),
		int(cVersion.release_version),
		C.GoString(cVersion.extra_version),
		C.GoString(cVersion.version),
		int(cVersion.version_id),
	}
}

func Config() PHPConfig {
	cConfig := C.frankenphp_get_config()

	return PHPConfig{
		Version:     Version(),
		ZTS:         bool(cConfig.zts),
		ZendSignals: bool(cConfig.zend_signals),
		ZendTimer:   bool(cConfig.zend_timer),
	}
}

// Init starts the PHP runtime and the configured workers.
func Init(options ...Option) error {
	if requestChan != nil {
		return AlreaydStartedError
	}

	opt := &opt{}
	for _, o := range options {
		if err := o(opt); err != nil {
			return err
		}
	}

	if opt.logger == nil {
		l, err := zap.NewDevelopment()
		if err != nil {
			return err
		}

		loggerMu.Lock()
		logger = l
		loggerMu.Unlock()
	} else {
		loggerMu.Lock()
		logger = opt.logger
		loggerMu.Unlock()
	}

	numCPU := runtime.NumCPU()

	var numWorkers int
	for i, w := range opt.workers {
		if w.num <= 0 {
			opt.workers[i].num = numCPU
		}

		numWorkers += opt.workers[i].num
	}

	if opt.numThreads <= 0 {
		if numWorkers >= numCPU {
			// Start at least as many threads as workers, and keep a free thread to handle requests in non-worker mode
			opt.numThreads = numWorkers + 1
		} else {
			opt.numThreads = numCPU
		}
	} else if opt.numThreads <= numWorkers {
		return NotEnoughThreads
	}

	config := Config()

	if config.Version.MajorVersion < 8 || config.Version.MinorVersion < 2 {
		return InvalidPHPVersionError
	}

	if config.ZTS {
		if !config.ZendTimer && runtime.GOOS == "linux" {
			logger.Warn(`Zend Timer is not enabled, "--enable-zend-timer" configuration option or timeouts (e.g. "max_execution_time") will not work as expected`)
		}
	} else {
		opt.numThreads = 1
		logger.Warn(`ZTS is not enabled, only 1 thread will be available, recompile PHP using the "--enable-zts" configuration option or performance will be degraded`)
	}

	shutdownWG.Add(1)
	requestChan = make(chan *http.Request)

	if C.frankenphp_init(C.int(opt.numThreads)) != 0 {
		return MainThreadCreationError
	}

	for _, w := range opt.workers {
		// TODO: start all the worker in parallell to reduce the boot time
		if err := startWorkers(w.fileName, w.num); err != nil {
			return err
		}
	}

	logger.Debug("FrankenPHP started")

	return nil
}

// Shutdown stops the workers and the PHP runtime.
func Shutdown() {
	stopWorkers()
	close(requestChan)
	shutdownWG.Wait()
	requestChan = nil

	logger.Debug("FrankenPHP shut down")
}

//export go_shutdown
func go_shutdown() {
	shutdownWG.Done()
}

func getLogger() *zap.Logger {
	loggerMu.RLock()
	defer loggerMu.RUnlock()

	return logger
}

func updateServerContext(request *http.Request, create bool) error {
	fc, ok := FromContext(request.Context())
	if !ok {
		return InvalidRequestError
	}

	var cAuthUser, cAuthPassword *C.char
	if fc.authPassword != "" {
		cAuthPassword = C.CString(fc.authPassword)
	}

	if authUser := fc.Env["REMOTE_USER"]; authUser != "" {
		cAuthUser = C.CString(authUser)
	}

	cMethod := C.CString(request.Method)
	cQueryString := C.CString(request.URL.RawQuery)
	contentLengthStr := request.Header.Get("Content-Length")
	contentLength := 0
	if contentLengthStr != "" {
		var err error
		contentLength, err = strconv.Atoi(contentLengthStr)
		if err != nil {
			return fmt.Errorf("invalid Content-Length header: %w", err)
		}
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

	var rh, mwrh cgo.Handle
	if fc.responseWriter == nil {
		mwrh = cgo.NewHandle(request)
	} else {
		rh = cgo.NewHandle(request)
	}

	ret := C.frankenphp_update_server_context(
		C.bool(create),
		C.uintptr_t(rh),
		C.uintptr_t(mwrh),

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

	if ret > 0 {
		return RequestContextCreationError
	}

	return nil
}

// ServeHTTP executes a PHP script according to the given context.
func ServeHTTP(responseWriter http.ResponseWriter, request *http.Request) error {
	shutdownWG.Add(1)
	defer shutdownWG.Done()

	fc, ok := FromContext(request.Context())
	if !ok {
		return InvalidRequestError
	}

	if err := populateEnv(request); err != nil {
		return err
	}

	fc.responseWriter = responseWriter
	fc.done = make(chan interface{})

	rc := requestChan
	// Detect if a worker is available to handle this request
	if nil == fc.responseWriter {
		fc.Env["FRANKENPHP_WORKER"] = "1"
	} else if v, ok := workersRequestChans.Load(fc.Env["SCRIPT_FILENAME"]); ok {
		fc.Env["FRANKENPHP_WORKER"] = "1"
		rc = v.(chan *http.Request)
	}

	if rc != nil {
		rc <- request
		<-fc.done
	}

	return nil
}

//export go_fetch_request
func go_fetch_request() C.uintptr_t {
	r, ok := <-requestChan
	if !ok {
		return 0
	}

	return C.uintptr_t(cgo.NewHandle(r))
}

func maybeCloseContext(fc *FrankenPHPContext) {
	fc.closed.Do(func() {
		close(fc.done)
	})
}

//export go_execute_script
func go_execute_script(rh unsafe.Pointer) {
	handle := cgo.Handle(rh)
	defer handle.Delete()

	request := handle.Value().(*http.Request)
	fc, ok := FromContext(request.Context())
	if !ok {
		panic(InvalidRequestError)
	}
	defer maybeCloseContext(fc)

	if err := updateServerContext(request, true); err != nil {
		panic(err)
	}

	if C.frankenphp_request_startup() < 0 {
		panic(RequestStartupError)
	}

	cFileName := C.CString(fc.Env["SCRIPT_FILENAME"])
	defer C.free(unsafe.Pointer(cFileName))

	if C.frankenphp_execute_script(cFileName) < 0 {
		panic(ScriptExecutionError)
	}

	C.frankenphp_clean_server_context()
	C.frankenphp_request_shutdown()
}

//export go_ub_write
func go_ub_write(rh C.uintptr_t, cString *C.char, length C.int) (C.size_t, C.bool) {
	r := cgo.Handle(rh).Value().(*http.Request)
	fc, _ := FromContext(r.Context())

	var writer io.Writer
	if fc.responseWriter == nil {
		var b bytes.Buffer
		// log the output of the worker
		writer = &b
	} else {
		writer = fc.responseWriter
	}

	i, _ := writer.Write([]byte(C.GoStringN(cString, length)))

	if fc.responseWriter == nil {
		fc.Logger.Info(writer.(*bytes.Buffer).String())
	}

	return C.size_t(i), C.bool(clientHasClosed(r))
}

//export go_register_variables
func go_register_variables(rh C.uintptr_t, trackVarsArray *C.zval) {
	var env map[string]string
	r := cgo.Handle(rh).Value().(*http.Request)
	env = r.Context().Value(contextKey).(*FrankenPHPContext).Env

	le := len(env) * 2
	cArr := (**C.char)(C.malloc(C.size_t(le) * C.size_t(unsafe.Sizeof((*C.char)(nil)))))
	defer C.free(unsafe.Pointer(cArr))

	variables := unsafe.Slice(cArr, le)

	var i int
	for k, v := range env {
		variables[i] = C.CString(k)
		i++

		variables[i] = C.CString(v)
		i++
	}

	C.frankenphp_register_bulk_variables(cArr, C.size_t(le), trackVarsArray)

	for _, v := range variables {
		C.free(unsafe.Pointer(v))
	}
}

//export go_add_header
func go_add_header(rh C.uintptr_t, cString *C.char, length C.int) {
	r := cgo.Handle(rh).Value().(*http.Request)
	fc := r.Context().Value(contextKey).(*FrankenPHPContext)

	parts := strings.SplitN(C.GoStringN(cString, length), ": ", 2)
	if len(parts) != 2 {
		fc.Logger.Debug("invalid header", zap.String("header", parts[0]))

		return
	}

	fc.responseWriter.Header().Add(parts[0], parts[1])
}

//export go_write_header
func go_write_header(rh C.uintptr_t, status C.int) {
	r := cgo.Handle(rh).Value().(*http.Request)
	fc := r.Context().Value(contextKey).(*FrankenPHPContext)

	if fc.responseWriter == nil {
		return
	}

	fc.responseWriter.WriteHeader(int(status))

	if status >= 100 && status < 200 {
		// Clear headers, it's not automatically done by ResponseWriter.WriteHeader() for 1xx responses
		h := fc.responseWriter.Header()
		for k := range h {
			delete(h, k)
		}
	}
}

//export go_sapi_flush
func go_sapi_flush(rh C.uintptr_t) bool {
	r := cgo.Handle(rh).Value().(*http.Request)
	fc := r.Context().Value(contextKey).(*FrankenPHPContext)

	if fc.responseWriter == nil {
		return true
	}

	flusher, ok := fc.responseWriter.(http.Flusher)
	if !ok {
		return true
	}

	if clientHasClosed(r) {
		return true
	}

	if r.ProtoMajor == 1 {
		if _, err := r.Body.Read(nil); err != nil {
			// Don't flush until the whole body has been read to prevent https://github.com/golang/go/issues/15527
			return false
		}
	}

	flusher.Flush()

	return false
}

//export go_read_post
func go_read_post(rh C.uintptr_t, cBuf *C.char, countBytes C.size_t) C.size_t {
	r := cgo.Handle(rh).Value().(*http.Request)

	p := make([]byte, int(countBytes))
	readBytes, err := r.Body.Read(p)
	if err != nil && err != io.EOF {
		// invalid Read on closed Body may happen because of https://github.com/golang/go/issues/15527
		fc, _ := FromContext(r.Context())
		fc.Logger.Error("error while reading the request body", zap.Error(err))
	}

	if readBytes != 0 {
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

//export go_log
func go_log(message *C.char, level C.int) {
	l := getLogger()
	m := C.GoString(message)

	var le syslogLevel
	if level < C.int(emerg) || level > C.int(debug) {
		le = info
	} else {
		le = syslogLevel(level)
	}

	switch le {
	case emerg, alert, crit, err:
		l.Error(m, zap.Stringer("syslog_level", syslogLevel(level)))

	case warning:
		l.Warn(m, zap.Stringer("syslog_level", syslogLevel(level)))

	case debug:
		l.Debug(m, zap.Stringer("syslog_level", syslogLevel(level)))

	default:
		l.Info(m, zap.Stringer("syslog_level", syslogLevel(level)))
	}
}
