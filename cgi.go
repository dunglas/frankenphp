package frankenphp

import "C"
import (
	"crypto/tls"
	"net"
	"net/http"
	"path/filepath"
	"strings"
)

type serverKey int

const (
	contentLength serverKey = iota
	documentRoot
	documentUri
	gatewayInterface
	httpHost
	https
	pathInfo
	phpSelf
	remoteAddr
	remoteHost
	remotePort
	requestScheme
	scriptFilename
	scriptName
	serverName
	serverPort
	serverProtocol
	serverSoftware
	sslProtocol
)

func allocServerVariable(cArr *[27]*C.char, env map[string]string, serverKey serverKey, envKey string, val string) {
	if val, ok := env[envKey]; ok {
		cArr[serverKey] = C.CString(val)
		delete(env, envKey)

		return
	}

	cArr[serverKey] = C.CString(val)
}

// computeKnownVariables returns a set of CGI environment variables for the request.
//
// TODO: handle this case https://github.com/caddyserver/caddy/issues/3718
// Inspired by https://github.com/caddyserver/caddy/blob/master/modules/caddyhttp/reverseproxy/fastcgi/fastcgi.go
func computeKnownVariables(request *http.Request) (cArr [27]*C.char) {
	fc, fcOK := FromContext(request.Context())
	if !fcOK {
		panic("not a FrankenPHP request")
	}

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

	ra, raOK := fc.env["REMOTE_ADDR"]
	if raOK {
		cArr[remoteAddr] = C.CString(ra)
		delete(fc.env, "REMOTE_ADDR")
	} else {
		cArr[remoteAddr] = C.CString(ip)
	}

	if rh, ok := fc.env["REMOTE_HOST"]; ok {
		cArr[remoteHost] = C.CString(rh) // For speed, remote host lookups disabled
		delete(fc.env, "REMOTE_HOST")
	} else {
		if raOK {
			cArr[remoteHost] = C.CString(ip)
		} else {
			cArr[remoteHost] = cArr[remoteAddr]
		}
	}

	allocServerVariable(&cArr, fc.env, remotePort, "REMOTE_PORT", port)
	allocServerVariable(&cArr, fc.env, documentRoot, "DOCUMENT_ROOT", fc.documentRoot)
	allocServerVariable(&cArr, fc.env, pathInfo, "PATH_INFO", fc.pathInfo)
	allocServerVariable(&cArr, fc.env, phpSelf, "PHP_SELF", request.URL.Path)
	allocServerVariable(&cArr, fc.env, documentUri, "DOCUMENT_URI", fc.docURI)
	allocServerVariable(&cArr, fc.env, scriptFilename, "SCRIPT_FILENAME", fc.scriptFilename)
	allocServerVariable(&cArr, fc.env, scriptName, "SCRIPT_NAME", fc.scriptName)

	var rs string
	if request.TLS == nil {
		rs = "http"
	} else {
		rs = "https"

		if h, ok := fc.env["HTTPS"]; ok {
			cArr[https] = C.CString(h)
			delete(fc.env, "HTTPS")
		} else {
			cArr[https] = C.CString("on")
		}

		// and pass the protocol details in a manner compatible with apache's mod_ssl
		// (which is why these have a SSL_ prefix and not TLS_).
		if p, ok := fc.env["SSL_PROTOCOL"]; ok {
			cArr[sslProtocol] = C.CString(p)
			delete(fc.env, "SSL_PROTOCOL")
		} else {
			if v, ok := tlsProtocolStrings[request.TLS.Version]; ok {
				cArr[sslProtocol] = C.CString(v)
			}
		}
	}
	allocServerVariable(&cArr, fc.env, requestScheme, "REQUEST_SCHEME", rs)

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

	allocServerVariable(&cArr, fc.env, serverName, "SERVER_NAME", reqHost)
	if reqPort != "" {
		allocServerVariable(&cArr, fc.env, serverPort, "SERVER_PORT", reqPort)
	}

	// Variables defined in CGI 1.1 spec
	// Some variables are unused but cleared explicitly to prevent
	// the parent environment from interfering.

	// These values can not be override
	cArr[contentLength] = C.CString(request.Header.Get("Content-Length"))

	allocServerVariable(&cArr, fc.env, gatewayInterface, "GATEWAY_INTERFACE", "CGI/1.1")
	allocServerVariable(&cArr, fc.env, serverProtocol, "SERVER_PROTOCOL", request.Proto)
	allocServerVariable(&cArr, fc.env, serverSoftware, "SERVER_SOFTWARE", "FrankenPHP")
	allocServerVariable(&cArr, fc.env, httpHost, "HTTP_HOST", request.Host) // added here, since not always part of headers

	return
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
