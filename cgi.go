package frankenphp

// #include <php_variables.h>
// #include "frankenphp.h"
import "C"
import (
	"crypto/tls"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"unsafe"

	"github.com/dunglas/frankenphp/internal/headers"
)

var knownServerKeys = []string{
	"CONTENT_LENGTH",
	"DOCUMENT_ROOT",
	"DOCUMENT_URI",
	"GATEWAY_INTERFACE",
	"HTTP_HOST",
	"HTTPS",
	"PATH_INFO",
	"PHP_SELF",
	"REMOTE_ADDR",
	"REMOTE_HOST",
	"REMOTE_PORT",
	"REQUEST_SCHEME",
	"SCRIPT_FILENAME",
	"SCRIPT_NAME",
	"SERVER_NAME",
	"SERVER_PORT",
	"SERVER_PROTOCOL",
	"SERVER_SOFTWARE",
	"SSL_PROTOCOL",
	"AUTH_TYPE",
	"REMOTE_IDENT",
	"CONTENT_TYPE",
	"PATH_TRANSLATED",
	"QUERY_STRING",
	"REMOTE_USER",
	"REQUEST_METHOD",
	"REQUEST_URI",
}

// computeKnownVariables returns a set of CGI environment variables for the request.
//
// TODO: handle this case https://github.com/caddyserver/caddy/issues/3718
// Inspired by https://github.com/caddyserver/caddy/blob/master/modules/caddyhttp/reverseproxy/fastcgi/fastcgi.go
func addKnownVariablesToServer(thread *phpThread, request *http.Request, fc *FrankenPHPContext, trackVarsArray *C.zval) {
	keys := getKnownVariableKeys(thread)
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

	var https string
	var sslProtocol string
	var rs string
	if request.TLS == nil {
		rs = "http"
		https = ""
		sslProtocol = ""
	} else {
		rs = "https"
		https = "on"

		// and pass the protocol details in a manner compatible with apache's mod_ssl
		// (which is why these have an SSL_ prefix and not TLS_).
		if v, ok := tlsProtocolStrings[request.TLS.Version]; ok {
			sslProtocol = v
		} else {
			sslProtocol = ""
		}
	}

	reqHost, reqPort, _ := net.SplitHostPort(request.Host)

	if reqHost == "" {
		// whatever, just assume there was no port
		reqHost = request.Host
	}

	if reqPort == "" {
		// compliance with the CGI specification requires that
		// the SERVER_PORT variable MUST be set to the TCP/IP port number on which this request is received from the client
		// even if the port is the default port for the scheme and could otherwise be omitted from a URI.
		// https://tools.ietf.org/html/rfc3875#section-4.1.15
		switch rs {
		case "https":
			reqPort = "443"
		case "http":
			reqPort = "80"
		}
	}

	serverPort := reqPort
	contentLength := request.Header.Get("Content-Length")

	var requestURI string
	if fc.originalRequest != nil {
		requestURI = fc.originalRequest.URL.RequestURI()
	} else {
		requestURI = request.URL.RequestURI()
	}

	C.frankenphp_register_bulk(
		trackVarsArray,
		packCgiVariable(keys["REMOTE_ADDR"], ip),
		packCgiVariable(keys["REMOTE_HOST"], ip),
		packCgiVariable(keys["REMOTE_PORT"], port),
		packCgiVariable(keys["DOCUMENT_ROOT"], fc.documentRoot),
		packCgiVariable(keys["PATH_INFO"], fc.pathInfo),
		packCgiVariable(keys["PHP_SELF"], request.URL.Path),
		packCgiVariable(keys["DOCUMENT_URI"], fc.docURI),
		packCgiVariable(keys["SCRIPT_FILENAME"], fc.scriptFilename),
		packCgiVariable(keys["SCRIPT_NAME"], fc.scriptName),
		packCgiVariable(keys["HTTPS"], https),
		packCgiVariable(keys["SSL_PROTOCOL"], sslProtocol),
		packCgiVariable(keys["REQUEST_SCHEME"], rs),
		packCgiVariable(keys["SERVER_NAME"], reqHost),
		packCgiVariable(keys["SERVER_PORT"], serverPort),
		// Variables defined in CGI 1.1 spec
		// Some variables are unused but cleared explicitly to prevent
		// the parent environment from interfering.
		// These values can not be overridden
		packCgiVariable(keys["CONTENT_LENGTH"], contentLength),
		packCgiVariable(keys["GATEWAY_INTERFACE"], "CGI/1.1"),
		packCgiVariable(keys["SERVER_PROTOCOL"], request.Proto),
		packCgiVariable(keys["SERVER_SOFTWARE"], "FrankenPHP"),
		packCgiVariable(keys["HTTP_HOST"], request.Host),
		// These values are always empty but must be defined:
		packCgiVariable(keys["AUTH_TYPE"], ""),
		packCgiVariable(keys["REMOTE_IDENT"], ""),
		// Request uri of the original request
		packCgiVariable(keys["REQUEST_URI"], requestURI),
	)

	// These values are already present in the SG(request_info), so we'll register them from there
	C.frankenphp_register_variables_from_request_info(
		trackVarsArray,
		keys["CONTENT_TYPE"],
		keys["PATH_TRANSLATED"],
		keys["QUERY_STRING"],
		keys["REMOTE_USER"],
		keys["REQUEST_METHOD"],
	)
}

func packCgiVariable(key *C.zend_string, value string) C.ht_key_value_pair {
	return C.ht_key_value_pair{key, toUnsafeChar(value), C.size_t(len(value))}
}

func addHeadersToServer(request *http.Request, thread *phpThread, fc *FrankenPHPContext, trackVarsArray *C.zval) {
	for field, val := range request.Header {
		if k := getCachedHeaderKey(thread, field); k != nil {
			v := strings.Join(val, ", ")
			C.frankenphp_register_single(k, toUnsafeChar(v), C.size_t(len(v)), trackVarsArray)
			continue
		}

		// if the header name could not be cached, register it inefficiently
		k := headers.GetUnCommonHeader(field)
		v := strings.Join(val, ", ")
		C.frankenphp_register_variable_safe(toUnsafeChar(k), toUnsafeChar(v), C.size_t(len(v)), trackVarsArray)
	}
}

func addPreparedEnvToServer(fc *FrankenPHPContext, trackVarsArray *C.zval) {
	for k, v := range fc.env {
		C.frankenphp_register_variable_safe(toUnsafeChar(k), toUnsafeChar(v), C.size_t(len(v)), trackVarsArray)
	}
	fc.env = nil
}

func getKnownVariableKeys(thread *phpThread) map[string]*C.zend_string {
	if thread.knownVariableKeys != nil {
		return thread.knownVariableKeys
	}
	threadServerKeys := make(map[string]*C.zend_string)
	for _, k := range knownServerKeys {
		threadServerKeys[k] = C.frankenphp_init_persistent_string(toUnsafeChar(k), C.size_t(len(k)))
	}
	thread.knownVariableKeys = threadServerKeys
	return threadServerKeys
}

func getCachedHeaderKey(thread *phpThread, key string) *C.zend_string {
	if thread.knownHeaderKeys == nil {
		thread.knownHeaderKeys = make(map[string]*C.zend_string)
	}

	// check if the header exists as cached zend_string
	if h, ok := thread.knownHeaderKeys[key]; ok {
		return h
	}

	// try to cache cache the header as zend_string
	commonKey := headers.GetCommonHeader(key)
	if commonKey == "" {
		return nil
	}

	zendHeader := C.frankenphp_init_persistent_string(toUnsafeChar(commonKey), C.size_t(len(commonKey)))
	thread.knownHeaderKeys[key] = zendHeader
	return zendHeader
}

//export go_register_variables
func go_register_variables(threadIndex C.uintptr_t, trackVarsArray *C.zval) {
	thread := phpThreads[threadIndex]
	r := thread.getActiveRequest()
	fc := r.Context().Value(contextKey).(*FrankenPHPContext)

	addKnownVariablesToServer(thread, r, fc, trackVarsArray)
	addHeadersToServer(r, thread, fc, trackVarsArray)

	// The Prepared Environment is registered last and can overwrite any previous values
	addPreparedEnvToServer(fc, trackVarsArray)
}

// splitPos returns the index where path should
// be split based on SplitPath.
//
// Adapted from https://github.com/caddyserver/caddy/blob/master/modules/caddyhttp/reverseproxy/fastcgi/fastcgi.go
// Copyright 2015 Matthew Holt and The Caddy Authors
func splitPos(fc *FrankenPHPContext, path string) int {
	if len(fc.splitPath) == 0 {
		return 0
	}

	lowerPath := strings.ToLower(path)
	for _, split := range fc.splitPath {
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
	// the trailing slash, so we need to re-add it afterward.
	// if the length is 1, then it's a path to the root,
	// and that should return ".", so we don't append the separator.
	if strings.HasSuffix(reqPath, "/") && len(reqPath) > 1 {
		path += separator
	}

	return path
}

const separator = string(filepath.Separator)

func toUnsafeChar(s string) *C.char {
	sData := unsafe.StringData(s)
	return (*C.char)(unsafe.Pointer(sData))
}
