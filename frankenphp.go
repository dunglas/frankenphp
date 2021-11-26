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

func ExecuteScript(responseWriter http.ResponseWriter, request *http.Request) error {
	if atomic.LoadInt32(&started) < 1 {
		if err := Startup(); err != nil {
			return err
		}
	}

	authPassword, err := populateEnv(request)
	if err != nil {
		return err
	}

	fc := request.Context().Value(FrankenPHPContextKey).(*FrankenPHPContext)

	var cAuthUser, cAuthPassword *C.char
	if authPassword != "" {
		cAuthPassword = C.CString(authPassword)
		defer C.free(unsafe.Pointer(cAuthPassword))
	}

	if authUser := fc.Env["REMOTE_USER"]; authUser != "" {
		cAuthUser = C.CString(authUser)
		defer C.free(unsafe.Pointer(cAuthUser))
	}

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
	if pathTranslated := fc.Env["PATH_TRANSLATED"]; pathTranslated != "" {
		cPathTranslated = C.CString(pathTranslated)
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

	cFileName := C.CString(fc.Env["SCRIPT_FILENAME"])
	defer C.free(unsafe.Pointer(cFileName))

	C.frankenphp_execute_script(cFileName)
	C.frankenphp_request_shutdown()

	return nil
}

// buildEnv returns a set of CGI environment variables for the request.
//
// TODO: handle this case https://github.com/caddyserver/caddy/issues/3718
// Inspired by https://github.com/caddyserver/caddy/blob/master/modules/caddyhttp/reverseproxy/fastcgi/fastcgi.go
func populateEnv(request *http.Request) (authPassword string, err error) {
	fc := request.Context().Value(FrankenPHPContextKey).(*FrankenPHPContext)

	_, addrOk := fc.Env["REMOTE_ADDR"]
	_, portOk := fc.Env["REMOTE_PORT"]
	if !addrOk || !portOk {
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

		if _, ok := fc.Env["REMOTE_ADDR"]; !ok {
			fc.Env["REMOTE_ADDR"] = ip
		}
		if _, ok := fc.Env["REMOTE_HOST"]; !ok {
			fc.Env["REMOTE_HOST"] = ip // For speed, remote host lookups disabled
		}
		if _, ok := fc.Env["REMOTE_PORT"]; !ok {
			fc.Env["REMOTE_PORT"] = port
		}
	}

	if _, ok := fc.Env["DOCUMENT_ROOT"]; !ok {
		// make sure file root is absolute
		root, err := filepath.Abs(fc.DocumentRoot)
		if err != nil {
			return "", err
		}

		if fc.ResolveRootSymlink {
			if root, err = filepath.EvalSymlinks(root); err != nil {
				return "", err
			}
		}

		fc.Env["DOCUMENT_ROOT"] = root
	}

	fpath := request.URL.Path
	scriptName := fpath

	docURI := fpath
	// split "actual path" from "path info" if configured
	if splitPos := splitPos(fc, fpath); splitPos > -1 {
		docURI = fpath[:splitPos]
		fc.Env["PATH_INFO"] = fpath[splitPos:]

		// Strip PATH_INFO from SCRIPT_NAME
		scriptName = strings.TrimSuffix(scriptName, fc.Env["PATH_INFO"])
	}

	// SCRIPT_FILENAME is the absolute path of SCRIPT_NAME
	scriptFilename := sanitizedPathJoin(fc.Env["DOCUMENT_ROOT"], scriptName)

	// Ensure the SCRIPT_NAME has a leading slash for compliance with RFC3875
	// Info: https://tools.ietf.org/html/rfc3875#section-4.1.13
	if scriptName != "" && !strings.HasPrefix(scriptName, "/") {
		scriptName = "/" + scriptName
	}

	if _, ok := fc.Env["DOCUMENT_URI"]; !ok {
		fc.Env["DOCUMENT_URI"] = docURI
	}
	if _, ok := fc.Env["SCRIPT_FILENAME"]; !ok {
		fc.Env["SCRIPT_FILENAME"] = scriptFilename
	}
	if _, ok := fc.Env["SCRIPT_NAME"]; !ok {
		fc.Env["SCRIPT_NAME"] = scriptName
	}

	if _, ok := fc.Env["REQUEST_SCHEME"]; !ok {
		if request.TLS == nil {
			fc.Env["REQUEST_SCHEME"] = "http"
		} else {
			fc.Env["REQUEST_SCHEME"] = "https"
		}
	}

	if request.TLS != nil {
		if _, ok := fc.Env["HTTPS"]; !ok {
			fc.Env["HTTPS"] = "on"
		}

		// and pass the protocol details in a manner compatible with apache's mod_ssl
		// (which is why these have a SSL_ prefix and not TLS_).
		_, sslProtocolOk := fc.Env["SSL_PROTOCOL"]
		v, versionOk := tlsProtocolStrings[request.TLS.Version]
		if !sslProtocolOk && versionOk {
			fc.Env["SSL_PROTOCOL"] = v
		}
	}

	_, serverNameOk := fc.Env["SERVER_NAME"]
	_, serverPortOk := fc.Env["SERVER_PORT"]
	if !serverNameOk || !serverPortOk {
		reqHost, reqPort, err := net.SplitHostPort(request.Host)
		if err == nil {
			if !serverNameOk {
				fc.Env["SERVER_NAME"] = reqHost
			}

			// compliance with the CGI specification requires that
			// SERVER_PORT should only exist if it's a valid numeric value.
			// Info: https://www.ietf.org/rfc/rfc3875 Page 18
			if !serverPortOk && reqPort != "" {
				fc.Env["SERVER_PORT"] = reqPort
			}
		} else if !serverNameOk {
			// whatever, just assume there was no port
			fc.Env["SERVER_NAME"] = request.Host
		}
	}

	// Variables defined in CGI 1.1 spec
	// Some variables are unused but cleared explicitly to prevent
	// the parent environment from interfering.
	// We never override an entry previously set
	if _, ok := fc.Env["REMOTE_IDENT"]; !ok {
		fc.Env["REMOTE_IDENT"] = "" // Not used
	}
	if _, ok := fc.Env["AUTH_TYPE"]; !ok {
		fc.Env["AUTH_TYPE"] = "" // Not used
	}
	if _, ok := fc.Env["CONTENT_LENGTH"]; !ok {
		fc.Env["CONTENT_LENGTH"] = request.Header.Get("Content-Length")
	}
	if _, ok := fc.Env["CONTENT_TYPE"]; !ok {
		fc.Env["CONTENT_TYPE"] = request.Header.Get("Content-Type")
	}
	if _, ok := fc.Env["GATEWAY_INTERFACE"]; !ok {
		fc.Env["GATEWAY_INTERFACE"] = "CGI/1.1"
	}
	if _, ok := fc.Env["QUERY_STRING"]; !ok {
		fc.Env["QUERY_STRING"] = request.URL.RawQuery
	}
	if _, ok := fc.Env["QUERY_STRING"]; !ok {
		fc.Env["QUERY_STRING"] = request.URL.RawQuery
	}
	if _, ok := fc.Env["REQUEST_METHOD"]; !ok {
		fc.Env["REQUEST_METHOD"] = request.Method
	}
	if _, ok := fc.Env["SERVER_PROTOCOL"]; !ok {
		fc.Env["SERVER_PROTOCOL"] = request.Proto
	}
	if _, ok := fc.Env["SERVER_SOFTWARE"]; !ok {
		fc.Env["SERVER_SOFTWARE"] = "FrankenPHP"
	}
	if _, ok := fc.Env["HTTP_HOST"]; !ok {
		fc.Env["HTTP_HOST"] = request.Host // added here, since not always part of headers
	}
	if _, ok := fc.Env["REQUEST_URI"]; !ok {
		fc.Env["REQUEST_URI"] = request.URL.RequestURI()
	}

	// compliance with the CGI specification requires that
	// PATH_TRANSLATED should only exist if PATH_INFO is defined.
	// Info: https://www.ietf.org/rfc/rfc3875 Page 14
	if fc.Env["PATH_INFO"] != "" {
		fc.Env["PATH_TRANSLATED"] = sanitizedPathJoin(fc.Env["DOCUMENT_ROOT"], fc.Env["PATH_INFO"]) // Info: http://www.oreilly.com/openbook/cgi/ch02_04.html
	}

	// Add all HTTP headers to env variables
	for field, val := range request.Header {
		k := "HTTP_" + headerNameReplacer.Replace(strings.ToUpper(field))
		if _, ok := fc.Env[k]; !ok {
			fc.Env[k] = strings.Join(val, ", ")
		}
	}

	if _, ok := fc.Env["REMOTE_USER"]; !ok {
		var (
			authUser string
			ok       bool
		)
		authUser, authPassword, ok = request.BasicAuth()
		if ok {
			fc.Env["REMOTE_USER"] = authUser
		}
	}

	return authPassword, nil
}

// splitPos returns the index where path should
// be split based on SplitPath.
//
// Adapted from https://github.com/caddyserver/caddy/blob/master/modules/caddyhttp/reverseproxy/fastcgi/fastcgi.go
// Copyright 2015 Matthew Holt and The Caddy Authors
func splitPos(fc *FrankenPHPContext, path string) int {
	if len(fc.SplitPath) == 0 {
		return 0
	}

	lowerPath := strings.ToLower(path)
	for _, split := range fc.SplitPath {
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
	for k, v := range r.Context().Value(FrankenPHPContextKey).(*FrankenPHPContext).Env {
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
