package frankenphp

// #cgo CFLAGS: -Wall -Wno-unused-variable
// #cgo CFLAGS: -I/usr/local/include/php -I/usr/local/include/php/Zend -I/usr/local/include/php/TSRM -I/usr/local/include/php/main
// #cgo LDFLAGS: -L/usr/local/lib -lphp
// #include <stdlib.h>
// #include <stdint.h>
// #include "php_variables.h"
// #include "frankenphp.h"
import "C"
import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"path/filepath"
	"runtime/cgo"
	"strconv"
	"strings"
	"sync/atomic"
	"unsafe"
)

var started int32

type CtxKey string

const CGICtxKey CtxKey = "cgi"

// FrankenPHP executes PHP scripts.
type FrankenPHP struct {
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

	// Extra environment variables.
	EnvVars map[string]string
}

func NewFrankenPHP() *FrankenPHP {
	return &FrankenPHP{SplitPath: []string{".php"}}
}

// Startup starts the PHP engine.
func Startup() error {
	if atomic.LoadInt32(&started) > 0 {
		return nil
	}

	if C.frankenphp_init() < 0 {
		return fmt.Errorf(`ZTS is not enabled, recompile PHP using the "--enable-zts" configuration option`)
	}
	atomic.StoreInt32(&started, 1)

	return nil
}

// Shutdown stops the PHP engine.
func Shutdown() {
	if atomic.LoadInt32(&started) < 1 {
		return
	}

	C.frankenphp_shutdown()
	atomic.StoreInt32(&started, 0)
}

func (f *FrankenPHP) ExecuteScript(documentRoot string, responseWriter http.ResponseWriter, request *http.Request, originalRequest *http.Request) error {
	if atomic.LoadInt32(&started) < 1 {
		if err := Startup(); err != nil {
			return err
		}
	}

	cgiEnv, err := f.buildCGIEnv(documentRoot, request, originalRequest)
	if err != nil {
		return err
	}

	var cAuthUser, cAuthPassword *C.char
	authUser, authPassword, ok := request.BasicAuth()
	if ok {
		cgiEnv["REMOTE_USER"] = authUser

		cAuthUser = C.CString(authUser)
		defer C.free(unsafe.Pointer(cAuthUser))

		cAuthPassword = C.CString(authPassword)
		defer C.free(unsafe.Pointer(cAuthPassword))
	}

	ctx := context.WithValue(request.Context(), CGICtxKey, cgiEnv)
	request = request.WithContext(ctx)

	wh := cgo.NewHandle(responseWriter)
	defer wh.Delete()

	rh := cgo.NewHandle(request)
	defer rh.Delete()

	cMethod := C.CString(request.Method)
	defer C.free(unsafe.Pointer(cMethod))

	cQueryString := C.CString(request.URL.RawQuery)
	defer C.free(unsafe.Pointer(cQueryString))

	contentLengthStr := request.Header.Get("Content-Length")
	contentLength := 0
	if contentLengthStr != "" {
		contentLength, _ = strconv.Atoi(contentLengthStr)
	}

	contentType := request.Header.Get("Content-Type")
	var cContentType *C.char
	if contentType != "" {
		cContentType = C.CString(contentType)
		defer C.free(unsafe.Pointer(cContentType))
	}

	var cPathTranslated *C.char
	if cgiEnv["PATH_TRANSLATED"] == "" {
		cPathTranslated = C.CString(cgiEnv["PATH_TRANSLATED"])
		defer C.free(unsafe.Pointer(cPathTranslated))
	}

	cRequestUri := C.CString(request.URL.RequestURI())
	defer C.free(unsafe.Pointer(cRequestUri))

	if C.frankenphp_request_startup(
		C.uintptr_t(wh),
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
	) < 0 {
		return fmt.Errorf("error during PHP request startup")
	}

	cFileName := C.CString(cgiEnv["SCRIPT_FILENAME"])
	defer C.free(unsafe.Pointer(cFileName))
	C.frankenphp_execute_script(cFileName)

	C.frankenphp_request_shutdown()

	return nil
}

// buildEnv returns a set of CGI environment variables for the request.
//
// TODO: handle this case https://github.com/caddyserver/caddy/issues/3718
// TODO: add Apache mod_ssl's like TLS versions
// Adapted from https://github.com/caddyserver/caddy/blob/master/modules/caddyhttp/reverseproxy/fastcgi/fastcgi.go
// Copyright 2015 Matthew Holt and The Caddy Authors
func (f FrankenPHP) buildCGIEnv(documentRoot string, request *http.Request, originalRequest *http.Request) (map[string]string, error) {
	if originalRequest == nil {
		originalRequest = request
	}

	var env map[string]string

	// Separate remote IP and port; more lenient than net.SplitHostPort
	var ip, port string
	if idx := strings.LastIndex(request.RemoteAddr, ":"); idx > -1 {
		ip = request.RemoteAddr[:idx]
		port = request.RemoteAddr[idx+1:]
	} else {
		ip = request.RemoteAddr
	}

	// Remove [] from IPv6 addresses
	ip = strings.Replace(ip, "[", "", 1)
	ip = strings.Replace(ip, "]", "", 1)

	// make sure file root is absolute
	root, err := filepath.Abs(documentRoot)
	if err != nil {
		return nil, err
	}

	if f.ResolveRootSymlink {
		if root, err = filepath.EvalSymlinks(root); err != nil {
			return nil, err
		}
	}

	fpath := request.URL.Path
	scriptName := fpath

	docURI := fpath
	// split "actual path" from "path info" if configured
	var pathInfo string
	if splitPos := f.splitPos(fpath); splitPos > -1 {
		docURI = fpath[:splitPos]
		pathInfo = fpath[splitPos:]

		// Strip PATH_INFO from SCRIPT_NAME
		scriptName = strings.TrimSuffix(scriptName, pathInfo)
	}

	// SCRIPT_FILENAME is the absolute path of SCRIPT_NAME
	scriptFilename := sanitizedPathJoin(root, scriptName)

	// Ensure the SCRIPT_NAME has a leading slash for compliance with RFC3875
	// Info: https://tools.ietf.org/html/rfc3875#section-4.1.13
	if scriptName != "" && !strings.HasPrefix(scriptName, "/") {
		scriptName = "/" + scriptName
	}

	requestScheme := "http"
	if request.TLS != nil {
		requestScheme = "https"
	}

	reqHost, reqPort, err := net.SplitHostPort(request.Host)
	if err != nil {
		// whatever, just assume there was no port
		reqHost = request.Host
	}

	// Some variables are unused but cleared explicitly to prevent
	// the parent environment from interfering.
	env = map[string]string{
		// Variables defined in CGI 1.1 spec
		"AUTH_TYPE":         "", // Not used
		"CONTENT_LENGTH":    request.Header.Get("Content-Length"),
		"CONTENT_TYPE":      request.Header.Get("Content-Type"),
		"GATEWAY_INTERFACE": "CGI/1.1",
		"PATH_INFO":         pathInfo,
		"QUERY_STRING":      request.URL.RawQuery,
		"REMOTE_ADDR":       ip,
		"REMOTE_HOST":       ip, // For speed, remote host lookups disabled
		"REMOTE_PORT":       port,
		"REMOTE_IDENT":      "", // Not used
		"REMOTE_USER":       "", // Will be set later
		"REQUEST_METHOD":    request.Method,
		"REQUEST_SCHEME":    requestScheme,
		"SERVER_NAME":       reqHost,
		"SERVER_PROTOCOL":   request.Proto,
		"SERVER_SOFTWARE":   "FrankenPHP",

		// Other variables
		"DOCUMENT_ROOT":   root,
		"DOCUMENT_URI":    docURI,
		"HTTP_HOST":       request.Host, // added here, since not always part of headers
		"REQUEST_URI":     originalRequest.URL.RequestURI(),
		"SCRIPT_FILENAME": scriptFilename,
		"SCRIPT_NAME":     scriptName,
	}

	// compliance with the CGI specification requires that
	// PATH_TRANSLATED should only exist if PATH_INFO is defined.
	// Info: https://www.ietf.org/rfc/rfc3875 Page 14
	if env["PATH_INFO"] != "" {
		env["PATH_TRANSLATED"] = sanitizedPathJoin(root, pathInfo) // Info: http://www.oreilly.com/openbook/cgi/ch02_04.html
	}

	// compliance with the CGI specification requires that
	// SERVER_PORT should only exist if it's a valid numeric value.
	// Info: https://www.ietf.org/rfc/rfc3875 Page 18
	if reqPort != "" {
		env["SERVER_PORT"] = reqPort
	}

	// Some web apps rely on knowing HTTPS or not
	if request.TLS != nil {
		env["HTTPS"] = "on"
		// and pass the protocol details in a manner compatible with apache's mod_ssl
		// (which is why these have a SSL_ prefix and not TLS_).
		v, ok := tlsProtocolStrings[request.TLS.Version]
		if ok {
			env["SSL_PROTOCOL"] = v
		}
	}

	// Add env variables from config
	for key, value := range f.EnvVars {
		env[key] = value
	}

	// Add all HTTP headers to env variables
	for field, val := range request.Header {
		header := strings.ToUpper(field)
		header = headerNameReplacer.Replace(header)
		env["HTTP_"+header] = strings.Join(val, ", ")
	}
	return env, nil
}

// splitPos returns the index where path should
// be split based on t.SplitPath.
//
// Adapted from https://github.com/caddyserver/caddy/blob/master/modules/caddyhttp/reverseproxy/fastcgi/fastcgi.go
// Copyright 2015 Matthew Holt and The Caddy Authors
func (f FrankenPHP) splitPos(path string) int {
	if len(f.SplitPath) == 0 {
		return 0
	}

	lowerPath := strings.ToLower(path)
	for _, split := range f.SplitPath {
		if idx := strings.Index(lowerPath, strings.ToLower(split)); idx > -1 {
			return idx + len(split)
		}
	}
	return -1
}

// Map of supported protocols to Apache ssl_mod format
// Note that these are slightly different from SupportedProtocols in caddytls/config.go
var tlsProtocolStrings = map[uint16]string{
	tls.VersionTLS10: "TLSv1",
	tls.VersionTLS11: "TLSv1.1",
	tls.VersionTLS12: "TLSv1.2",
	tls.VersionTLS13: "TLSv1.3",
}

var headerNameReplacer = strings.NewReplacer(" ", "_", "-", "_")

// SanitizedPathJoin performs filepath.Join(root, reqPath) that
// is safe against directory traversal attacks. It uses logic
// similar to that in the Go standard library, specifically
// in the implementation of http.Dir. The root is assumed to
// be a trusted path, but reqPath is not; and the output will
// never be outside of root. The resulting path can be used
// with the local file system.
//
// Adapted from https://github.com/caddyserver/caddy/blob/master/modules/caddyhttp/reverseproxy/fastcgi/fastcgi.go
// Copyright 2015 Matthew Holt and The Caddy Authors
func sanitizedPathJoin(root, reqPath string) string {
	if root == "" {
		root = "."
	}

	path := filepath.Join(root, filepath.Clean("/"+reqPath))

	// filepath.Join also cleans the path, and cleaning strips
	// the trailing slash, so we need to re-add it afterwards.
	// if the length is 1, then it's a path to the root,
	// and that should return ".", so we don't append the separator.
	if strings.HasSuffix(reqPath, "/") && len(reqPath) > 1 {
		path += separator
	}

	return path
}

const separator = string(filepath.Separator)

//export go_ub_write
func go_ub_write(wh C.uintptr_t, cString *C.char, length C.int) C.size_t {
	w := cgo.Handle(wh).Value().(http.ResponseWriter)
	i, _ := w.Write([]byte(C.GoStringN(cString, length)))

	return C.size_t(i)
}

//export go_register_variables
func go_register_variables(rh C.uintptr_t, trackVarsArray *C.zval) {
	r := cgo.Handle(rh).Value().(*http.Request)
	for k, v := range r.Context().Value(CGICtxKey).(map[string]string) {
		ck := C.CString(k)
		cv := C.CString(v)
		C.php_register_variable_safe(ck, cv, C.size_t(len(v)), trackVarsArray)

		C.free(unsafe.Pointer(ck))
		C.free(unsafe.Pointer(cv))
	}
}

//export go_add_header
func go_add_header(wh C.uintptr_t, cString *C.char, length C.int) {
	w := cgo.Handle(wh).Value().(http.ResponseWriter)

	parts := strings.SplitN(C.GoStringN(cString, length), ": ", 2)
	if len(parts) != 2 {
		log.Printf(`invalid header "%s"`+"\n", parts[0])

		return
	}

	w.Header().Add(parts[0], parts[1])
}

//export go_write_header
func go_write_header(wh C.uintptr_t, status C.int) {
	w := cgo.Handle(wh).Value().(http.ResponseWriter)
	w.WriteHeader(int(status))
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
