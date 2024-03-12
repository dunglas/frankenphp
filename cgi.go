package frankenphp

// #include "frankenphp.h"
import "C"
import (
	"crypto/tls"
	"net"
	"net/http"
	"path/filepath"
	"runtime"
	"strings"
	"unsafe"
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
}

func setKnownServerVariable(p *runtime.Pinner, cArr *[27]C.go_string, serverKey serverKey, val string) {
	if val == "" {
		return
	}

	valData := unsafe.StringData(val)
	p.Pin(valData)
	cArr[serverKey].len = C.size_t(len(val))
	cArr[serverKey].data = (*C.char)(unsafe.Pointer(valData))
}

// computeKnownVariables returns a set of CGI environment variables for the request.
//
// TODO: handle this case https://github.com/caddyserver/caddy/issues/3718
// Inspired by https://github.com/caddyserver/caddy/blob/master/modules/caddyhttp/reverseproxy/fastcgi/fastcgi.go
func computeKnownVariables(request *http.Request, p *runtime.Pinner) (cArr [27]C.go_string) {
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

	ra, raOK := fc.env["REMOTE_ADDR\x00"]
	if raOK {
		setKnownServerVariable(p, &cArr, remoteAddr, ra)
	} else {
		setKnownServerVariable(p, &cArr, remoteAddr, ip)
	}

	if rh, ok := fc.env["REMOTE_HOST\x00"]; ok {
		setKnownServerVariable(p, &cArr, remoteHost, rh) // For speed, remote host lookups disabled
	} else {
		if raOK {
			setKnownServerVariable(p, &cArr, remoteHost, ip)
		} else {
			cArr[remoteHost] = cArr[remoteAddr]
		}
	}

	setKnownServerVariable(p, &cArr, remotePort, port)
	setKnownServerVariable(p, &cArr, documentRoot, fc.documentRoot)
	setKnownServerVariable(p, &cArr, pathInfo, fc.pathInfo)
	setKnownServerVariable(p, &cArr, phpSelf, request.URL.Path)
	setKnownServerVariable(p, &cArr, documentUri, fc.docURI)
	setKnownServerVariable(p, &cArr, scriptFilename, fc.scriptFilename)
	setKnownServerVariable(p, &cArr, scriptName, fc.scriptName)

	var rs string
	if request.TLS == nil {
		rs = "http"
	} else {
		rs = "https"

		if h, ok := fc.env["HTTPS\x00"]; ok {
			setKnownServerVariable(p, &cArr, https, h)
		} else {
			setKnownServerVariable(p, &cArr, https, "on")
		}

		// and pass the protocol details in a manner compatible with apache's mod_ssl
		// (which is why these have a SSL_ prefix and not TLS_).
		if pr, ok := fc.env["SSL_PROTOCOL\x00"]; ok {
			setKnownServerVariable(p, &cArr, sslProtocol, pr)
		} else {
			if v, ok := tlsProtocolStrings[request.TLS.Version]; ok {
				setKnownServerVariable(p, &cArr, sslProtocol, v)
			}
		}
	}

	setKnownServerVariable(p, &cArr, requestScheme, rs)
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

	setKnownServerVariable(p, &cArr, serverName, reqHost)
	if reqPort != "" {
		setKnownServerVariable(p, &cArr, serverPort, reqPort)
	}

	// Variables defined in CGI 1.1 spec
	// Some variables are unused but cleared explicitly to prevent
	// the parent environment from interfering.

	// These values can not be override
	setKnownServerVariable(p, &cArr, contentLength, request.Header.Get("Content-Length"))
	setKnownServerVariable(p, &cArr, gatewayInterface, "CGI/1.1")
	setKnownServerVariable(p, &cArr, serverProtocol, request.Proto)
	setKnownServerVariable(p, &cArr, serverSoftware, "FrankenPHP")
	setKnownServerVariable(p, &cArr, httpHost, request.Host) // added here, since not always part of headers

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
