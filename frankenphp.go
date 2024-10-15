// Package frankenphp embeds PHP in Go projects and provides a SAPI for net/http.
//
// This is the core of the [FrankenPHP app server], and can be used in any Go program.
//
// [FrankenPHP app server]: https://frankenphp.dev
package frankenphp

// Use PHP includes corresponding to your PHP installation by running:
//
//   export CGO_CFLAGS=$(php-config --includes)
//   export CGO_LDFLAGS="$(php-config --ldflags) $(php-config --libs)"
//
// We also set these flags for hardening: https://github.com/docker-library/php/blob/master/8.2/bookworm/zts/Dockerfile#L57-L59

// #cgo darwin pkg-config: libxml-2.0
// #cgo CFLAGS: -Wall -Werror
// #cgo CFLAGS: -I/usr/local/include/php -I/usr/local/include/php/main -I/usr/local/include/php/TSRM -I/usr/local/include/php/Zend -I/usr/local/include/php/ext -I/usr/local/include/php/ext/date/lib
// #cgo linux CFLAGS: -D_GNU_SOURCE
// #cgo darwin LDFLAGS: -L/opt/homebrew/opt/libiconv/lib -liconv
// #cgo linux LDFLAGS: -lresolv
// #cgo LDFLAGS: -L/usr/local/lib -L/usr/lib -lphp -ldl -lm -lutil
// #include <stdlib.h>
// #include <stdint.h>
// #include <php_variables.h>
// #include <zend_llist.h>
// #include <SAPI.h>
// #include "frankenphp.h"
import "C"
import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/maypok86/otter"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	// debug on Linux
	//_ "github.com/ianlancetaylor/cgosymbolizer"
)

type contextKeyStruct struct{}

var contextKey = contextKeyStruct{}

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
	done        chan struct{}
	shutdownWG  sync.WaitGroup

	loggerMu sync.RWMutex
	logger   *zap.Logger

	metrics Metrics = nullMetrics{}
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
	documentRoot string
	splitPath    []string
	env          PreparedEnv
	logger       *zap.Logger

	docURI         string
	pathInfo       string
	scriptName     string
	scriptFilename string

	// Whether the request is already closed by us
	closed sync.Once

	responseWriter http.ResponseWriter
	exitStatus     C.int

	done      chan interface{}
	startedAt time.Time
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
func NewRequestWithContext(r *http.Request, opts ...RequestOption) (*http.Request, error) {
	fc := &FrankenPHPContext{
		done: make(chan interface{}),
	}
	for _, o := range opts {
		if err := o(fc); err != nil {
			return nil, err
		}
	}

	if fc.documentRoot == "" {
		if EmbeddedAppPath != "" {
			fc.documentRoot = EmbeddedAppPath
		} else {
			var err error
			if fc.documentRoot, err = os.Getwd(); err != nil {
				return nil, err
			}
		}
	}

	if fc.splitPath == nil {
		fc.splitPath = []string{".php"}
	}

	if fc.env == nil {
		fc.env = make(map[string]string)
	}

	if fc.logger == nil {
		fc.logger = getLogger()
	}

	if splitPos := splitPos(fc, r.URL.Path); splitPos > -1 {
		fc.docURI = r.URL.Path[:splitPos]
		fc.pathInfo = r.URL.Path[splitPos:]

		// Strip PATH_INFO from SCRIPT_NAME
		fc.scriptName = strings.TrimSuffix(r.URL.Path, fc.pathInfo)

		// Ensure the SCRIPT_NAME has a leading slash for compliance with RFC3875
		// Info: https://tools.ietf.org/html/rfc3875#section-4.1.13
		if fc.scriptName != "" && !strings.HasPrefix(fc.scriptName, "/") {
			fc.scriptName = "/" + fc.scriptName
		}
	}

	// SCRIPT_FILENAME is the absolute path of SCRIPT_NAME
	fc.scriptFilename = sanitizedPathJoin(fc.documentRoot, fc.scriptName)

	c := context.WithValue(r.Context(), contextKey, fc)

	return r.WithContext(c), nil
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
	Version                PHPVersion
	ZTS                    bool
	ZendSignals            bool
	ZendMaxExecutionTimers bool
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
		Version:                Version(),
		ZTS:                    bool(cConfig.zts),
		ZendSignals:            bool(cConfig.zend_signals),
		ZendMaxExecutionTimers: bool(cConfig.zend_max_execution_timers),
	}
}

// MaxThreads is internally used during tests. It is written to, but never read and may go away in the future.
var MaxThreads int

func calculateMaxThreads(opt *opt) error {
	maxProcs := runtime.GOMAXPROCS(0) * 2

	var numWorkers int
	for i, w := range opt.workers {
		if w.num <= 0 {
			// https://github.com/dunglas/frankenphp/issues/126
			opt.workers[i].num = maxProcs
		}
		metrics.TotalWorkers(w.fileName, w.num)

		numWorkers += opt.workers[i].num
	}

	if opt.numThreads <= 0 {
		if numWorkers >= maxProcs {
			// Start at least as many threads as workers, and keep a free thread to handle requests in non-worker mode
			opt.numThreads = numWorkers + 1
		} else {
			opt.numThreads = maxProcs
		}
	} else if opt.numThreads <= numWorkers {
		return NotEnoughThreads
	}

	metrics.TotalThreads(opt.numThreads)
	MaxThreads = opt.numThreads

	return nil
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

	if opt.metrics != nil {
		metrics = opt.metrics
	}

	err := calculateMaxThreads(opt)
	if err != nil {
		return err
	}

	config := Config()

	if config.Version.MajorVersion < 8 || (config.Version.MajorVersion == 8 && config.Version.MinorVersion < 2) {
		return InvalidPHPVersionError
	}

	if config.ZTS {
		if !config.ZendMaxExecutionTimers && runtime.GOOS == "linux" {
			logger.Warn(`Zend Max Execution Timers are not enabled, timeouts (e.g. "max_execution_time") are disabled, recompile PHP with the "--enable-zend-max-execution-timers" configuration option to fix this issue`)
		}
	} else {
		opt.numThreads = 1
		logger.Warn(`ZTS is not enabled, only 1 thread will be available, recompile PHP using the "--enable-zts" configuration option or performance will be degraded`)
	}

	shutdownWG.Add(1)
	done = make(chan struct{})
	requestChan = make(chan *http.Request)
	initPHPThreads(opt.numThreads)

	if C.frankenphp_init(C.int(opt.numThreads)) != 0 {
		return MainThreadCreationError
	}

	if err := initWorkers(opt.workers); err != nil {
		return err
	}

	if err := restartWorkersOnFileChanges(opt.workers); err != nil {
		return err
	}

	if c := logger.Check(zapcore.InfoLevel, "FrankenPHP started 🐘"); c != nil {
		c.Write(zap.String("php_version", Version().Version), zap.Int("num_threads", opt.numThreads))
	}
	if EmbeddedAppPath != "" {
		if c := logger.Check(zapcore.InfoLevel, "embedded PHP app 📦"); c != nil {
			c.Write(zap.String("path", EmbeddedAppPath))
		}
	}

	return nil
}

// Shutdown stops the workers and the PHP runtime.
func Shutdown() {
	drainWorkers()
	drainThreads()
	metrics.Shutdown()
	requestChan = nil

	// Remove the installed app
	if EmbeddedAppPath != "" {
		os.RemoveAll(EmbeddedAppPath)
	}

	logger.Debug("FrankenPHP shut down")
}

//export go_shutdown
func go_shutdown() {
	shutdownWG.Done()
}

func drainThreads() {
	close(done)
	shutdownWG.Wait()
	phpThreads = nil
}

func getLogger() *zap.Logger {
	loggerMu.RLock()
	defer loggerMu.RUnlock()

	return logger
}

func updateServerContext(request *http.Request, create bool, isWorkerRequest bool) error {
	fc, ok := FromContext(request.Context())
	if !ok {
		return InvalidRequestError
	}

	authUser, authPassword, ok := request.BasicAuth()
	var cAuthUser, cAuthPassword *C.char
	if ok && authPassword != "" {
		cAuthPassword = C.CString(authPassword)
	}
	if ok && authUser != "" {
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

	// compliance with the CGI specification requires that
	// PATH_TRANSLATED should only exist if PATH_INFO is defined.
	// Info: https://www.ietf.org/rfc/rfc3875 Page 14
	var cPathTranslated *C.char
	if fc.pathInfo != "" {
		cPathTranslated = C.CString(sanitizedPathJoin(fc.documentRoot, fc.pathInfo)) // Info: http://www.oreilly.com/openbook/cgi/ch02_04.html
	}

	cRequestUri := C.CString(request.URL.RequestURI())
	isBootingAWorkerScript := fc.responseWriter == nil

	ret := C.frankenphp_update_server_context(
		C.bool(create),
		C.bool(isWorkerRequest || isBootingAWorkerScript),
		C.bool(!isBootingAWorkerScript),

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

	fc.responseWriter = responseWriter
	fc.startedAt = time.Now()

	isWorker := fc.responseWriter == nil
	isWorkerRequest := false

	rc := requestChan
	// Detect if a worker is available to handle this request
	if !isWorker {
		if worker, ok := workers[fc.scriptFilename]; ok {
			isWorkerRequest = true
			metrics.StartWorkerRequest(fc.scriptFilename)
			rc = worker.requestChan
		} else {
			metrics.StartRequest()
		}
	}

	select {
	case <-done:
	case rc <- request:
		<-fc.done
	}

	if !isWorker {
		if isWorkerRequest {
			metrics.StopWorkerRequest(fc.scriptFilename, time.Since(fc.startedAt))
		} else {
			metrics.StopRequest()
		}
	}

	return nil
}

//export go_putenv
func go_putenv(str *C.char, length C.int) C.bool {
	// Create a byte slice from C string with a specified length
	s := C.GoBytes(unsafe.Pointer(str), length)

	// Convert byte slice to string
	envString := string(s)

	// Check if '=' is present in the string
	if idx := strings.IndexByte(envString, '='); idx != -1 && idx < len(envString)-1 {
		// '=' found, set the environment variable
		key := envString[:idx]
		val := envString[idx+1:]

		err := os.Setenv(key, val)
		if err != nil {
			return false // Failure
		}
	} else {
		// No '=', unset the environment variable
		err := os.Unsetenv(envString)
		if err != nil {
			return false // Failure
		}
	}

	return true // Success
}

//export go_getenv
func go_getenv(name *C.char, length C.int, value **C.char, value_len *C.int) C.bool {
	if name == nil {
		// Get all environment variables
		env := os.Environ()
		// Concatenate them with null separators
		concatenatedEnv := strings.Join(env, "\x00") + "\x00" // Add an extra null terminator at the end
		// Convert to C string
		*value = C.CString(concatenatedEnv)
		// Set the length of the concatenated string
		*value_len = C.int(len(concatenatedEnv))
		return true // Success
	}

	// Create a byte slice from C string with a specified length
	nameBytes := C.GoBytes(unsafe.Pointer(name), length)

	// Convert byte slice to string
	envName := string(nameBytes)

	// Get the environment variable value
	envValue, exists := os.LookupEnv(envName)
	if !exists {
		// Environment variable does not exist
		*value = nil
		*value_len = 0
		return false // Return 0 to indicate failure
	}

	// Convert Go string to C string
	cValue := C.CString(envValue)
	*value = cValue
	*value_len = C.int(len(envValue))

	return true // Return 1 to indicate success
}

//export go_handle_request
func go_handle_request(threadIndex C.uintptr_t) bool {
	select {
	case <-done:
		return false

	case r := <-requestChan:
		thread := phpThreads[threadIndex]
		thread.mainRequest = r

		fc, ok := FromContext(r.Context())
		if !ok {
			panic(InvalidRequestError)
		}
		defer func() {
			maybeCloseContext(fc)
			thread.mainRequest = nil
		}()

		if err := updateServerContext(r, true, false); err != nil {
			panic(err)
		}

		// scriptFilename is freed in frankenphp_execute_script()
		fc.exitStatus = C.frankenphp_execute_script(C.CString(fc.scriptFilename))
		if fc.exitStatus < 0 {
			panic(ScriptExecutionError)
		}

		return true
	}
}

func maybeCloseContext(fc *FrankenPHPContext) {
	fc.closed.Do(func() {
		close(fc.done)
	})
}

//export go_ub_write
func go_ub_write(threadIndex C.uintptr_t, cBuf *C.char, length C.int) (C.size_t, C.bool) {
	r := phpThreads[threadIndex].getActiveRequest()
	fc, _ := FromContext(r.Context())

	var writer io.Writer
	if fc.responseWriter == nil {
		var b bytes.Buffer
		// log the output of the worker
		writer = &b
	} else {
		writer = fc.responseWriter
	}

	i, e := writer.Write(unsafe.Slice((*byte)(unsafe.Pointer(cBuf)), length))
	if e != nil {
		if c := fc.logger.Check(zapcore.ErrorLevel, "write error"); c != nil {
			c.Write(zap.Error(e))
		}
	}

	if fc.responseWriter == nil {
		fc.logger.Info(writer.(*bytes.Buffer).String())
	}

	return C.size_t(i), C.bool(clientHasClosed(r))
}

// There are around 60 common request headers according to https://en.wikipedia.org/wiki/List_of_HTTP_header_fields#Request_fields
// Give some space for custom headers
var headerKeyCache = func() otter.Cache[string, string] {
	c, err := otter.MustBuilder[string, string](256).Build()
	if err != nil {
		panic(err)
	}

	return c
}()

//export go_register_variables
func go_register_variables(threadIndex C.uintptr_t, trackVarsArray *C.zval) {
	thread := phpThreads[threadIndex]
	r := thread.getActiveRequest()
	fc := r.Context().Value(contextKey).(*FrankenPHPContext)

	dynamicVariables := make([]C.php_variable, len(fc.env)+len(r.Header))

	var l int

	// Add all HTTP headers to env variables
	for field, val := range r.Header {
		k, ok := headerKeyCache.Get(field)
		if !ok {
			k = "HTTP_" + headerNameReplacer.Replace(strings.ToUpper(field)) + "\x00"
			headerKeyCache.SetIfAbsent(field, k)
		}

		if _, ok := fc.env[k]; ok {
			continue
		}

		v := strings.Join(val, ", ")

		kData := unsafe.StringData(k)
		vData := unsafe.StringData(v)

		thread.Pin(kData)
		thread.Pin(vData)

		dynamicVariables[l]._var = (*C.char)(unsafe.Pointer(kData))
		dynamicVariables[l].data_len = C.size_t(len(v))
		dynamicVariables[l].data = (*C.char)(unsafe.Pointer(vData))

		l++
	}

	for k, v := range fc.env {
		if _, ok := knownServerKeys[k]; ok {
			continue
		}

		kData := unsafe.StringData(k)
		vData := unsafe.Pointer(unsafe.StringData(v))

		thread.Pin(kData)
		thread.Pin(vData)

		dynamicVariables[l]._var = (*C.char)(unsafe.Pointer(kData))
		dynamicVariables[l].data_len = C.size_t(len(v))
		dynamicVariables[l].data = (*C.char)(unsafe.Pointer(vData))

		l++
	}

	knownVariables := computeKnownVariables(r, &thread.Pinner)

	dvsd := unsafe.SliceData(dynamicVariables)
	thread.Pin(dvsd)

	C.frankenphp_register_bulk_variables(&knownVariables[0], dvsd, C.size_t(l), trackVarsArray)

	thread.Unpin()

	fc.env = nil
}

//export go_apache_request_headers
func go_apache_request_headers(threadIndex C.uintptr_t, hasActiveRequest bool) (*C.go_string, C.size_t) {
	thread := phpThreads[threadIndex]

	if !hasActiveRequest {
		// worker mode, not handling a request
		mfc := thread.mainRequest.Context().Value(contextKey).(*FrankenPHPContext)

		if c := mfc.logger.Check(zapcore.DebugLevel, "apache_request_headers() called in non-HTTP context"); c != nil {
			c.Write(zap.String("worker", mfc.scriptFilename))
		}

		return nil, 0
	}
	r := thread.getActiveRequest()

	headers := make([]C.go_string, 0, len(r.Header)*2)

	for field, val := range r.Header {
		fd := unsafe.StringData(field)
		thread.Pin(fd)

		cv := strings.Join(val, ", ")
		vd := unsafe.StringData(cv)
		thread.Pin(vd)

		headers = append(
			headers,
			C.go_string{C.size_t(len(field)), (*C.char)(unsafe.Pointer(fd))},
			C.go_string{C.size_t(len(cv)), (*C.char)(unsafe.Pointer(vd))},
		)
	}

	sd := unsafe.SliceData(headers)
	thread.Pin(sd)

	return sd, C.size_t(len(r.Header))
}

//export go_apache_request_cleanup
func go_apache_request_cleanup(threadIndex C.uintptr_t) {
	phpThreads[threadIndex].Unpin()
}

func addHeader(fc *FrankenPHPContext, cString *C.char, length C.int) {
	parts := strings.SplitN(C.GoStringN(cString, length), ": ", 2)
	if len(parts) != 2 {
		if c := fc.logger.Check(zapcore.DebugLevel, "invalid header"); c != nil {
			c.Write(zap.String("header", parts[0]))
		}

		return
	}

	fc.responseWriter.Header().Add(parts[0], parts[1])
}

//export go_write_headers
func go_write_headers(threadIndex C.uintptr_t, status C.int, headers *C.zend_llist) {
	r := phpThreads[threadIndex].getActiveRequest()
	fc := r.Context().Value(contextKey).(*FrankenPHPContext)

	if fc.responseWriter == nil {
		return
	}

	current := headers.head
	for current != nil {
		h := (*C.sapi_header_struct)(unsafe.Pointer(&(current.data)))

		addHeader(fc, h.header, C.int(h.header_len))
		current = current.next
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
func go_sapi_flush(threadIndex C.uintptr_t) bool {
	r := phpThreads[threadIndex].getActiveRequest()
	fc := r.Context().Value(contextKey).(*FrankenPHPContext)

	if fc.responseWriter == nil || clientHasClosed(r) {
		return true
	}

	if err := http.NewResponseController(fc.responseWriter).Flush(); err != nil {
		if c := fc.logger.Check(zapcore.ErrorLevel, "the current responseWriter is not a flusher"); c != nil {
			c.Write(zap.Error(err))
		}
	}

	return false
}

//export go_read_post
func go_read_post(threadIndex C.uintptr_t, cBuf *C.char, countBytes C.size_t) (readBytes C.size_t) {
	r := phpThreads[threadIndex].getActiveRequest()

	p := unsafe.Slice((*byte)(unsafe.Pointer(cBuf)), countBytes)
	var err error
	for readBytes < countBytes && err == nil {
		var n int
		n, err = r.Body.Read(p[readBytes:])
		readBytes += C.size_t(n)
	}

	return
}

//export go_read_cookies
func go_read_cookies(threadIndex C.uintptr_t) *C.char {
	r := phpThreads[threadIndex].getActiveRequest()

	cookies := r.Cookies()
	if len(cookies) == 0 {
		return nil
	}
	cookieStrings := make([]string, len(cookies))
	for i, cookie := range cookies {
		cookieStrings[i] = cookie.String()
	}

	// freed in frankenphp_free_request_context()
	return C.CString(strings.Join(cookieStrings, "; "))
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
		if c := l.Check(zapcore.ErrorLevel, m); c != nil {
			c.Write(zap.Stringer("syslog_level", syslogLevel(level)))
		}

	case warning:
		if c := l.Check(zapcore.WarnLevel, m); c != nil {
			c.Write(zap.Stringer("syslog_level", syslogLevel(level)))
		}

	case debug:
		if c := l.Check(zapcore.DebugLevel, m); c != nil {
			c.Write(zap.Stringer("syslog_level", syslogLevel(level)))
		}

	default:
		if c := l.Check(zapcore.InfoLevel, m); c != nil {
			c.Write(zap.Stringer("syslog_level", syslogLevel(level)))
		}
	}
}

// ExecuteScriptCLI executes the PHP script passed as parameter.
// It returns the exit status code of the script.
func ExecuteScriptCLI(script string, args []string) int {
	cScript := C.CString(script)
	defer C.free(unsafe.Pointer(cScript))

	argc, argv := convertArgs(args)
	defer freeArgs(argv)

	return int(C.frankenphp_execute_script_cli(cScript, argc, (**C.char)(unsafe.Pointer(&argv[0]))))
}

func convertArgs(args []string) (C.int, []*C.char) {
	argc := C.int(len(args))
	argv := make([]*C.char, argc)
	for i, arg := range args {
		argv[i] = C.CString(arg)
	}
	return argc, argv
}

func freeArgs(argv []*C.char) {
	for _, arg := range argv {
		C.free(unsafe.Pointer(arg))
	}
}

func executePHPFunction(functionName string) {
	cFunctionName := C.CString(functionName)
	defer C.free(unsafe.Pointer(cFunctionName))

	success := C.frankenphp_execute_php_function(cFunctionName)

	if success == 1 {
		if c := logger.Check(zapcore.DebugLevel, "php function call successful"); c != nil {
			c.Write(zap.String("function", functionName))
		}
	} else {
		if c := logger.Check(zapcore.ErrorLevel, "php function call failed"); c != nil {
			c.Write(zap.String("function", functionName))
		}
	}
}
