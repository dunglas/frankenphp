package frankenphp

import (
	"crypto/tls"
	"net"
	"net/http"
	"path/filepath"
	"strings"
)

// populateEnv returns a set of CGI environment variables for the request.
//
// TODO: handle this case https://github.com/caddyserver/caddy/issues/3718
// Inspired by https://github.com/caddyserver/caddy/blob/master/modules/caddyhttp/reverseproxy/fastcgi/fastcgi.go
func populateEnv(request *http.Request) error {
	fc, ok := FromContext(request.Context())
	if !ok {
		panic("not a FrankenPHP request")
	}

	if fc.populated {
		return nil
	}

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
			return err
		}

		if fc.ResolveRootSymlink {
			if root, err = filepath.EvalSymlinks(root); err != nil {
				return err
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
	
	if _, ok := fc.Env["PHP_SELF"]; !ok {
		fc.Env["PHP_SELF"] = fpath
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
		authUser, fc.authPassword, ok = request.BasicAuth()
		if ok {
			fc.Env["REMOTE_USER"] = authUser
		}
	}

	fc.populated = true

	return nil
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
