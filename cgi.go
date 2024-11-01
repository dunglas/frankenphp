package frankenphp

//#include <php_variables.h>
//#include "frankenphp.h"
import "C"
import (
	"crypto/tls"
	"net"
	"net/http"
	"path/filepath"
	"strings"
)

var knownServerKeys = map[string]struct{}{
	"CONTENT_LENGTH\x00":    {},
	"DOCUMENT_ROOT\x00":     {},
	"DOCUMENT_URI\x00":      {},
	"GATEWAY_INTERFACE\x00": {},
	"HTTP_HOST\x00":         {},
	"HTTPS\x00":             {},
	"PATH_INFO\x00":         {},
	"PHP_SELF\x00":          {},
	"REMOTE_ADDR\x00":       {},
	"REMOTE_HOST\x00":       {},
	"REMOTE_PORT\x00":       {},
	"REQUEST_SCHEME\x00":    {},
	"SCRIPT_FILENAME\x00":   {},
	"SCRIPT_NAME\x00":       {},
	"SERVER_NAME\x00":       {},
	"SERVER_PORT\x00":       {},
	"SERVER_PROTOCOL\x00":   {},
	"SERVER_SOFTWARE\x00":   {},
	"SSL_PROTOCOL\x00":      {},
	"AUTH_TYPE\x00":         {},
	"REMOTE_IDENT\x00":      {},
	"CONTENT_TYPE\x00":      {},
	"PATH_TRANSLATED\x00":   {},
	"QUERY_STRING\x00":      {},
	"REMOTE_USER\x00":       {},
	"REQUEST_METHOD\x00":    {},
	"REQUEST_URI\x00":       {},
}

// computeKnownVariables returns a set of CGI environment variables for the request.
//
// TODO: handle this case https://github.com/caddyserver/caddy/issues/3718
// Inspired by https://github.com/caddyserver/caddy/blob/master/modules/caddyhttp/reverseproxy/fastcgi/fastcgi.go
func addKnownVariablesToServer(thread *phpThread, request *http.Request, fc *FrankenPHPContext, track_vars_array *C.zval) {
	knownServerKeys := getKnownVariableKeys(thread)
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

	ra, raOK := fc.env["REMOTE_ADDR\x00"]
	if raOK {
		registerTrustedVar(knownServerKeys["REMOTE_ADDR\x00"], ra, track_vars_array, thread)
	} else {
		registerTrustedVar(knownServerKeys["REMOTE_ADDR\x00"], ip, track_vars_array, thread)
	}

	if rh, ok := fc.env["REMOTE_HOST\x00"]; ok {
		registerTrustedVar(knownServerKeys["REMOTE_HOST\x00"], rh, track_vars_array, thread) // For speed, remote host lookups disabled
	} else {
		if raOK {
			registerTrustedVar(knownServerKeys["REMOTE_HOST\x00"], ra, track_vars_array, thread)
		} else {
			registerTrustedVar(knownServerKeys["REMOTE_HOST\x00"], ip, track_vars_array, thread)
		}
	}

	registerTrustedVar(knownServerKeys["REMOTE_PORT\x00"], port, track_vars_array, thread)
	registerTrustedVar(knownServerKeys["DOCUMENT_ROOT\x00"], fc.documentRoot, track_vars_array, thread)
	registerTrustedVar(knownServerKeys["PATH_INFO\x00"], fc.pathInfo, track_vars_array, thread)
	registerTrustedVar(knownServerKeys["PHP_SELF\x00"], request.URL.Path, track_vars_array, thread)
	registerTrustedVar(knownServerKeys["DOCUMENT_URI\x00"], fc.docURI, track_vars_array, thread)
	registerTrustedVar(knownServerKeys["SCRIPT_FILENAME\x00"], fc.scriptFilename, track_vars_array, thread)
	registerTrustedVar(knownServerKeys["SCRIPT_NAME\x00"], fc.scriptName, track_vars_array, thread)

	var rs string
	if request.TLS == nil {
		rs = "http"
		registerTrustedVar(knownServerKeys["HTTPS\x00"], "", track_vars_array, thread)
		registerTrustedVar(knownServerKeys["SSL_PROTOCOL\x00"], "", track_vars_array, thread)
	} else {
		rs = "https"

		if h, ok := fc.env["HTTPS\x00"]; ok {
			registerTrustedVar(knownServerKeys["HTTPS\x00"], h, track_vars_array, thread)
		} else {
			registerTrustedVar(knownServerKeys["HTTPS\x00"], "on", track_vars_array, thread)
		}

		// and pass the protocol details in a manner compatible with apache's mod_ssl
		// (which is why these have a SSL_ prefix and not TLS_).
		if pr, ok := fc.env["SSL_PROTOCOL\x00"]; ok {
			registerTrustedVar(knownServerKeys["SSL_PROTOCOL\x00"], pr, track_vars_array, thread)
		} else {
			if v, ok := tlsProtocolStrings[request.TLS.Version]; ok {
				registerTrustedVar(knownServerKeys["SSL_PROTOCOL\x00"], v, track_vars_array, thread)
			} else {
				registerTrustedVar(knownServerKeys["SSL_PROTOCOL\x00"], "", track_vars_array, thread)
			}
		}
	}

	registerTrustedVar(knownServerKeys["REQUEST_SCHEME\x00"], rs, track_vars_array, thread)
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

	registerTrustedVar(knownServerKeys["SERVER_NAME\x00"], reqHost, track_vars_array, thread)
	if reqPort != "" {
		registerTrustedVar(knownServerKeys["SERVER_PORT\x00"], reqPort, track_vars_array, thread)
	} else {
		registerTrustedVar(knownServerKeys["SERVER_PORT\x00"], "", track_vars_array, thread)
	}

	// Variables defined in CGI 1.1 spec
	// Some variables are unused but cleared explicitly to prevent
	// the parent environment from interfering.

	// These values can not be overridden
	registerTrustedVar(knownServerKeys["CONTENT_LENGTH\x00"], request.Header.Get("Content-Length"), track_vars_array, thread)
	registerTrustedVar(knownServerKeys["GATEWAY_INTERFACE\x00"], "CGI/1.1", track_vars_array, thread)
	registerTrustedVar(knownServerKeys["SERVER_PROTOCOL\x00"], request.Proto, track_vars_array, thread)
	registerTrustedVar(knownServerKeys["SERVER_SOFTWARE\x00"], "FrankenPHP", track_vars_array, thread)
	registerTrustedVar(knownServerKeys["HTTP_HOST\x00"], request.Host, track_vars_array, thread) // added here, since not always part of headers

	// These values are always empty but must be defined:
	registerTrustedVar(knownServerKeys["AUTH_TYPE\x00"], "", track_vars_array, thread)
	registerTrustedVar(knownServerKeys["REMOTE_IDENT\x00"], "", track_vars_array, thread)

	// These values are already present in the SG(request_info), so we'll register them from there
	C.frankenphp_register_variables_from_request_info(
		track_vars_array,
		knownServerKeys["CONTENT_TYPE\x00"],
		knownServerKeys["PATH_TRANSLATED\x00"],
		knownServerKeys["QUERY_STRING\x00"],
		knownServerKeys["REMOTE_USER\x00"],
		knownServerKeys["REQUEST_METHOD\x00"],
		knownServerKeys["REQUEST_URI\x00"],
	)
}

func registerTrustedVar(key *C.zend_string, value string, track_vars_array *C.zval, thread *phpThread) {
	C.frankenphp_register_trusted_var(key, thread.pinString(value), C.int(len(value)), track_vars_array)
}

func addHeadersToServer(thread *phpThread, request *http.Request, fc *FrankenPHPContext, track_vars_array *C.zval) {
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
		C.frankenphp_register_variable_safe(thread.pinString(k), thread.pinString(v), C.size_t(len(v)), track_vars_array)
	}
}

func addPreparedEnvToServer(thread *phpThread, fc *FrankenPHPContext, track_vars_array *C.zval) {
	for k, v := range fc.env {
		C.frankenphp_register_variable_safe(thread.pinString(k), thread.pinString(v), C.size_t(len(v)), track_vars_array)
	}
	fc.env = nil
}

func getKnownVariableKeys(thread *phpThread) map[string]*C.zend_string {
	if thread.knownVariableKeys != nil {
		return thread.knownVariableKeys
	}
	threadServerKeys := make(map[string]*C.zend_string)
	for k, _ := range knownServerKeys {
		keyWithoutNull := k[:len(k)-1]
		threadServerKeys[k] = C.frankenphp_init_interned_string(thread.pinString(keyWithoutNull), C.size_t(len(keyWithoutNull)))
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
	addHeadersToServer(thread, r, fc, trackVarsArray)
	addPreparedEnvToServer(thread, fc, trackVarsArray)
}

//export go_frankenphp_release_known_variable_keys
func go_frankenphp_release_known_variable_keys(thread_index C.uintptr_t) {
	thread := phpThreads[thread_index]
	if thread.knownVariableKeys == nil {
		return
	}
	for _, v := range thread.knownVariableKeys {
		C.frankenphp_release_zend_string(v)
	}
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
	// the trailing slash, so we need to re-add it afterwards.
	// if the length is 1, then it's a path to the root,
	// and that should return ".", so we don't append the separator.
	if strings.HasSuffix(reqPath, "/") && len(reqPath) > 1 {
		path += separator
	}

	return path
}

const separator = string(filepath.Separator)
