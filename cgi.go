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

	var serverPort string
	if reqPort != "" {
		serverPort = reqPort
	} else {
		serverPort = ""
	}

	contentLength := request.Header.Get("Content-Length")

	var requestURI string
	if fc.originalRequest != nil {
		requestURI = fc.originalRequest.URL.RequestURI()
	} else {
		requestURI = request.URL.RequestURI()
	}

	C.fphp_register_bulk_vars(
		keys["REMOTE_ADDR"], toUnsafeChar(ip), C.size_t(len(ip)),
		keys["REMOTE_HOST"], toUnsafeChar(ip), C.size_t(len(ip)),
		keys["REMOTE_PORT"], toUnsafeChar(port), C.size_t(len(port)),
		keys["DOCUMENT_ROOT"], toUnsafeChar(fc.documentRoot), C.size_t(len(fc.documentRoot)),
		keys["PATH_INFO"], toUnsafeChar(fc.pathInfo), C.size_t(len(fc.pathInfo)),
		keys["PHP_SELF"], toUnsafeChar(request.URL.Path), C.size_t(len(request.URL.Path)),
		keys["DOCUMENT_URI"], toUnsafeChar(fc.docURI), C.size_t(len(fc.docURI)),
		keys["SCRIPT_FILENAME"], toUnsafeChar(fc.scriptFilename), C.size_t(len(fc.scriptFilename)),
		keys["SCRIPT_NAME"], toUnsafeChar(fc.scriptName), C.size_t(len(fc.scriptName)),
		keys["HTTPS"], toUnsafeChar(https), C.size_t(len(https)),
		keys["SSL_PROTOCOL"], toUnsafeChar(sslProtocol), C.size_t(len(sslProtocol)),
		keys["REQUEST_SCHEME"], toUnsafeChar(rs), C.size_t(len(rs)),
		keys["SERVER_NAME"], toUnsafeChar(reqHost), C.size_t(len(reqHost)),
		keys["SERVER_PORT"], toUnsafeChar(serverPort), C.size_t(len(serverPort)),
		// Variables defined in CGI 1.1 spec
		// Some variables are unused but cleared explicitly to prevent
		// the parent environment from interfering.
		// These values can not be overridden
		keys["CONTENT_LENGTH"], toUnsafeChar(contentLength), C.size_t(len(contentLength)),
		keys["GATEWAY_INTERFACE"], toUnsafeChar("CGI/1.1"), C.size_t(len("CGI/1.1")),
		keys["SERVER_PROTOCOL"], toUnsafeChar(request.Proto), C.size_t(len(request.Proto)),
		keys["SERVER_SOFTWARE"], toUnsafeChar("FrankenPHP"), C.size_t(len("FrankenPHP")),
		keys["HTTP_HOST"], toUnsafeChar(request.Host), C.size_t(len(request.Host)),
		// These values are always empty but must be defined:
		keys["AUTH_TYPE"], nil, C.size_t(0),
		keys["REMOTE_IDENT"], nil, C.size_t(0),
		// Request uri of the original request
		keys["REQUEST_URI"], toUnsafeChar(requestURI), C.size_t(len(requestURI)),
		trackVarsArray,
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

func addHeadersToServer(request *http.Request, fc *FrankenPHPContext, trackVarsArray *C.zval) {
	for field, val := range request.Header {
		k, ok := headerKeyCache.Get(field)
		if !ok {
			k = "HTTP_" + headerNameReplacer.Replace(strings.ToUpper(field)) + "\x00"
			headerKeyCache.SetIfAbsent(field, k)
		}

		if _, ok := fc.env[k]; ok {
			continue
		}

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

//export go_register_variables
func go_register_variables(threadIndex C.uintptr_t, trackVarsArray *C.zval) {
	thread := phpThreads[threadIndex]
	r := thread.getActiveRequest()
	fc := r.Context().Value(contextKey).(*FrankenPHPContext)

	addKnownVariablesToServer(thread, r, fc, trackVarsArray)
	addHeadersToServer(r, fc, trackVarsArray)
	addPreparedEnvToServer(fc, trackVarsArray)
}

//export go_frankenphp_release_known_variable_keys
func go_frankenphp_release_known_variable_keys(threadIndex C.uintptr_t) {
	thread := phpThreads[threadIndex]
	if thread.knownVariableKeys == nil {
		return
	}
	for _, v := range thread.knownVariableKeys {
		C.frankenphp_release_zend_string(v)
	}
	thread.knownVariableKeys = nil
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
