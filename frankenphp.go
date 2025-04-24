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

// #cgo nocallback frankenphp_update_server_context
// #cgo noescape frankenphp_update_server_context
// #cgo darwin pkg-config: libxml-2.0
// #cgo CFLAGS: -Wall -Werror
// #cgo CFLAGS: -I/usr/local/include -I/usr/local/include/php -I/usr/local/include/php/main -I/usr/local/include/php/TSRM -I/usr/local/include/php/Zend -I/usr/local/include/php/ext -I/usr/local/include/php/ext/date/lib
// #cgo linux CFLAGS: -D_GNU_SOURCE
// #cgo darwin LDFLAGS: -Wl,-rpath,/usr/local/lib -L/opt/homebrew/opt/libiconv/lib -liconv
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
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	// debug on Linux
	//_ "github.com/ianlancetaylor/cgosymbolizer"
)

type contextKeyStruct struct{}

var contextKey = contextKeyStruct{}

var (
	ErrInvalidRequest         = errors.New("not a FrankenPHP request")
	ErrAlreadyStarted         = errors.New("FrankenPHP is already started")
	ErrInvalidPHPVersion      = errors.New("FrankenPHP is only compatible with PHP 8.2+")
	ErrMainThreadCreation     = errors.New("error creating the main thread")
	ErrRequestContextCreation = errors.New("error during request context creation")
	ErrScriptExecution        = errors.New("error during PHP script execution")
	ErrNotRunning             = errors.New("FrankenPHP is not running. For proper configuration visit: https://frankenphp.dev/docs/config/#caddyfile-config")

	isRunning bool

	loggerMu sync.RWMutex
	logger   *zap.Logger

	metrics Metrics = nullMetrics{}

	maxWaitTime time.Duration
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

func calculateMaxThreads(opt *opt) (int, int, int, error) {
	maxProcs := runtime.GOMAXPROCS(0) * 2

	var numWorkers int
	for i, w := range opt.workers {
		if w.num <= 0 {
			// https://github.com/dunglas/frankenphp/issues/126
			opt.workers[i].num = maxProcs
		}
		metrics.TotalWorkers(w.name, w.num)

		numWorkers += opt.workers[i].num
	}

	numThreadsIsSet := opt.numThreads > 0
	maxThreadsIsSet := opt.maxThreads != 0
	maxThreadsIsAuto := opt.maxThreads < 0 // maxthreads < 0 signifies auto mode (see phpmaintread.go)

	if numThreadsIsSet && !maxThreadsIsSet {
		opt.maxThreads = opt.numThreads
		if opt.numThreads <= numWorkers {
			err := fmt.Errorf("num_threads (%d) must be greater than the number of worker threads (%d)", opt.numThreads, numWorkers)
			return 0, 0, 0, err
		}

		return opt.numThreads, numWorkers, opt.maxThreads, nil
	}

	if maxThreadsIsSet && !numThreadsIsSet {
		opt.numThreads = numWorkers + 1
		if !maxThreadsIsAuto && opt.numThreads > opt.maxThreads {
			err := fmt.Errorf("max_threads (%d) must be greater than the number of worker threads (%d)", opt.maxThreads, numWorkers)
			return 0, 0, 0, err
		}

		return opt.numThreads, numWorkers, opt.maxThreads, nil
	}

	if !numThreadsIsSet {
		if numWorkers >= maxProcs {
			// Start at least as many threads as workers, and keep a free thread to handle requests in non-worker mode
			opt.numThreads = numWorkers + 1
		} else {
			opt.numThreads = maxProcs
		}
		opt.maxThreads = opt.numThreads

		return opt.numThreads, numWorkers, opt.maxThreads, nil
	}

	// both num_threads and max_threads are set
	if opt.numThreads <= numWorkers {
		err := fmt.Errorf("num_threads (%d) must be greater than the number of worker threads (%d)", opt.numThreads, numWorkers)
		return 0, 0, 0, err
	}

	if !maxThreadsIsAuto && opt.maxThreads < opt.numThreads {
		err := fmt.Errorf("max_threads (%d) must be greater than or equal to num_threads (%d)", opt.maxThreads, opt.numThreads)
		return 0, 0, 0, err
	}

	return opt.numThreads, numWorkers, opt.maxThreads, nil
}

// Init starts the PHP runtime and the configured workers.
func Init(options ...Option) error {
	if isRunning {
		return ErrAlreadyStarted
	}
	isRunning = true

	// Ignore all SIGPIPE signals to prevent weird issues with systemd: https://github.com/dunglas/frankenphp/issues/1020
	// Docker/Moby has a similar hack: https://github.com/moby/moby/blob/d828b032a87606ae34267e349bf7f7ccb1f6495a/cmd/dockerd/docker.go#L87-L90
	signal.Ignore(syscall.SIGPIPE)

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

	maxWaitTime = opt.maxWaitTime

	totalThreadCount, workerThreadCount, maxThreadCount, err := calculateMaxThreads(opt)
	if err != nil {
		return err
	}

	metrics.TotalThreads(totalThreadCount)
	MaxThreads = totalThreadCount

	config := Config()

	if config.Version.MajorVersion < 8 || (config.Version.MajorVersion == 8 && config.Version.MinorVersion < 2) {
		return ErrInvalidPHPVersion
	}

	if config.ZTS {
		if !config.ZendMaxExecutionTimers && runtime.GOOS == "linux" {
			logger.Warn(`Zend Max Execution Timers are not enabled, timeouts (e.g. "max_execution_time") are disabled, recompile PHP with the "--enable-zend-max-execution-timers" configuration option to fix this issue`)
		}
	} else {
		totalThreadCount = 1
		logger.Warn(`ZTS is not enabled, only 1 thread will be available, recompile PHP using the "--enable-zts" configuration option or performance will be degraded`)
	}

	mainThread, err := initPHPThreads(totalThreadCount, maxThreadCount, opt.phpIni)
	if err != nil {
		return err
	}

	regularRequestChan = make(chan *frankenPHPContext, totalThreadCount-workerThreadCount)
	regularThreads = make([]*phpThread, 0, totalThreadCount-workerThreadCount)
	for i := 0; i < totalThreadCount-workerThreadCount; i++ {
		convertToRegularThread(getInactivePHPThread())
	}

	if err := initWorkers(opt.workers); err != nil {
		return err
	}

	initAutoScaling(mainThread)

	if c := logger.Check(zapcore.InfoLevel, "FrankenPHP started 🐘"); c != nil {
		c.Write(zap.String("php_version", Version().Version), zap.Int("num_threads", mainThread.numThreads), zap.Int("max_threads", mainThread.maxThreads))
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
	if !isRunning {
		return
	}

	drainWatcher()
	drainAutoScaling()
	drainPHPThreads()

	metrics.Shutdown()

	// Remove the installed app
	if EmbeddedAppPath != "" {
		_ = os.RemoveAll(EmbeddedAppPath)
	}

	isRunning = false
	logger.Debug("FrankenPHP shut down")
}

func updateServerContext(thread *phpThread, fc *frankenPHPContext, isWorkerRequest bool) error {
	request := fc.request
	authUser, authPassword, ok := request.BasicAuth()
	var cAuthUser, cAuthPassword *C.char
	if ok && authPassword != "" {
		cAuthPassword = thread.pinCString(authPassword)
	}
	if ok && authUser != "" {
		cAuthUser = thread.pinCString(authUser)
	}

	cMethod := thread.pinCString(request.Method)
	cQueryString := thread.pinCString(request.URL.RawQuery)
	contentLengthStr := request.Header.Get("Content-Length")
	contentLength := 0
	if contentLengthStr != "" {
		var err error
		contentLength, err = strconv.Atoi(contentLengthStr)
		if err != nil || contentLength < 0 {
			return fmt.Errorf("invalid Content-Length header: %w", err)
		}
	}

	contentType := request.Header.Get("Content-Type")
	var cContentType *C.char
	if contentType != "" {
		cContentType = thread.pinCString(contentType)
	}

	// compliance with the CGI specification requires that
	// PATH_TRANSLATED should only exist if PATH_INFO is defined.
	// Info: https://www.ietf.org/rfc/rfc3875 Page 14
	var cPathTranslated *C.char
	if fc.pathInfo != "" {
		cPathTranslated = thread.pinCString(sanitizedPathJoin(fc.documentRoot, fc.pathInfo)) // Info: http://www.oreilly.com/openbook/cgi/ch02_04.html
	}

	cRequestUri := thread.pinCString(request.URL.RequestURI())

	ret := C.frankenphp_update_server_context(
		C.bool(isWorkerRequest || fc.responseWriter == nil),

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
		return ErrRequestContextCreation
	}

	return nil
}

// ServeHTTP executes a PHP script according to the given context.
func ServeHTTP(responseWriter http.ResponseWriter, request *http.Request) error {
	if !isRunning {
		return ErrNotRunning
	}

	fc, ok := fromContext(request.Context())
	if !ok {
		return ErrInvalidRequest
	}

	fc.responseWriter = responseWriter

	if !fc.validate() {
		return nil
	}

	// Detect if a worker is available to handle this request
	if worker, ok := workers[getWorkerKey(fc.workerName, fc.scriptFilename)]; ok {
		worker.handleRequest(fc)
		return nil
	}

	// If no worker was available, send the request to non-worker threads
	handleRequestWithRegularPHPThreads(fc)
	return nil
}

//export go_ub_write
func go_ub_write(threadIndex C.uintptr_t, cBuf *C.char, length C.int) (C.size_t, C.bool) {
	fc := phpThreads[threadIndex].getRequestContext()

	if fc.isDone {
		return 0, C.bool(true)
	}

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
		// probably starting a worker script, log the output
		fc.logger.Info(writer.(*bytes.Buffer).String())
	}

	return C.size_t(i), C.bool(fc.clientHasClosed())
}

//export go_apache_request_headers
func go_apache_request_headers(threadIndex C.uintptr_t) (*C.go_string, C.size_t) {
	thread := phpThreads[threadIndex]
	fc := thread.getRequestContext()

	if fc.responseWriter == nil {
		// worker mode, not handling a request

		if c := logger.Check(zapcore.DebugLevel, "apache_request_headers() called in non-HTTP context"); c != nil {
			c.Write(zap.String("worker", fc.scriptFilename))
		}

		return nil, 0
	}

	headers := make([]C.go_string, 0, len(fc.request.Header)*2)

	for field, val := range fc.request.Header {
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

	return sd, C.size_t(len(fc.request.Header))
}

func addHeader(fc *frankenPHPContext, cString *C.char, length C.int) {
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
func go_write_headers(threadIndex C.uintptr_t, status C.int, headers *C.zend_llist) C.bool {
	fc := phpThreads[threadIndex].getRequestContext()

	if fc.isDone {
		return C.bool(false)
	}

	if fc.responseWriter == nil {
		// probably starting a worker script, pretend we wrote headers so PHP still calls ub_write
		return C.bool(true)
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

	return C.bool(true)
}

//export go_sapi_flush
func go_sapi_flush(threadIndex C.uintptr_t) bool {
	fc := phpThreads[threadIndex].getRequestContext()
	if fc == nil || fc.responseWriter == nil {
		return false
	}

	if fc.clientHasClosed() && !fc.isDone {
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
	fc := phpThreads[threadIndex].getRequestContext()

	if fc.responseWriter == nil {
		return 0
	}

	p := unsafe.Slice((*byte)(unsafe.Pointer(cBuf)), countBytes)
	var err error
	for readBytes < countBytes && err == nil {
		var n int
		n, err = fc.request.Body.Read(p[readBytes:])
		readBytes += C.size_t(n)
	}

	return
}

//export go_read_cookies
func go_read_cookies(threadIndex C.uintptr_t) *C.char {
	cookies := phpThreads[threadIndex].getRequestContext().request.Header.Values("Cookie")
	cookie := strings.Join(cookies, "; ")
	if cookie == "" {
		return nil
	}

	// remove potential null bytes
	cookie = strings.ReplaceAll(cookie, "\x00", "")

	// freed in frankenphp_free_request_context()
	return C.CString(cookie)
}

//export go_log
func go_log(message *C.char, level C.int) {
	m := C.GoString(message)

	var le syslogLevel
	if level < C.int(emerg) || level > C.int(debug) {
		le = info
	} else {
		le = syslogLevel(level)
	}

	switch le {
	case emerg, alert, crit, err:
		if c := logger.Check(zapcore.ErrorLevel, m); c != nil {
			c.Write(zap.Stringer("syslog_level", syslogLevel(level)))
		}

	case warning:
		if c := logger.Check(zapcore.WarnLevel, m); c != nil {
			c.Write(zap.Stringer("syslog_level", syslogLevel(level)))
		}

	case debug:
		if c := logger.Check(zapcore.DebugLevel, m); c != nil {
			c.Write(zap.Stringer("syslog_level", syslogLevel(level)))
		}

	default:
		if c := logger.Check(zapcore.InfoLevel, m); c != nil {
			c.Write(zap.Stringer("syslog_level", syslogLevel(level)))
		}
	}
}

//export go_is_context_done
func go_is_context_done(threadIndex C.uintptr_t) C.bool {
	return C.bool(phpThreads[threadIndex].getRequestContext().isDone)
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

func timeoutChan(timeout time.Duration) <-chan time.Time {
	if timeout == 0 {
		return nil
	}

	return time.After(timeout)
}
