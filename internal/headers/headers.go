package headers

import (
	"strings"
)

// Translate header names to PHP header names
// All headers in 'commonHeaders' can be cached and registered safely
// All other headers must be prefixed with 'HTTP_' and sanitized
var headerNameReplacer = strings.NewReplacer(" ", "_", "-", "_")

var commonRequestHeaders = map[string]string{
	// Common headers
	"Accept":                    "HTTP_ACCEPT",
	"Accept-Charset":            "HTTP_ACCEPT_CHARSET",
	"Accept-Encoding":           "HTTP_ACCEPT_ENCODING",
	"Accept-Language":           "HTTP_ACCEPT_LANGUAGE",
	"Accept-Datetime":           "HTTP_ACCEPT_DATETIME",
	"Authorization":             "HTTP_AUTHORIZATION",
	"Cache-Control":             "HTTP_CACHE_CONTROL",
	"Connection":                "HTTP_CONNECTION",
	"Content-Length":            "HTTP_CONTENT_LENGTH",
	"Content-MD5":               "HTTP_CONTENT_MD5",
	"User-Agent":                "HTTP_USER_AGENT",
	"Referer":                   "HTTP_REFERER",
	"Cookie":                    "HTTP_COOKIE",
	"Host":                      "HTTP_HOST",
	"Content-Type":              "HTTP_CONTENT_TYPE",
	"Origin":                    "HTTP_ORIGIN",
	"Upgrade-Insecure-Requests": "HTTP_UPGRADE_INSECURE_REQUESTS",
	"If-None-Match":             "HTTP_IF_NONE_MATCH",
	"If-Modified-Since":         "HTTP_IF_MODIFIED_SINCE",
	"Transfer-Encoding":         "HTTP_TRANSFER_ENCODING",
	"Upgrade":                   "HTTP_UPGRADE",
	"DNT":                       "HTTP_DNT",
	// Browser security headers
	"Sec-Fetch-Dest":             "HTTP_SEC_FETCH_DEST",
	"Sec-Fetch-Mode":             "HTTP_SEC_FETCH_MODE",
	"Sec-Fetch-Site":             "HTTP_SEC_FETCH_SITE",
	"Sec-Fetch-User":             "HTTP_SEC_FETCH_USER",
	"Sec-Ch-Ua":                  "HTTP_SEC_CH_UA",
	"Sec-Ch-Ua-Mobile":           "HTTP_SEC_CH_UA_MOBILE",
	"Sec-Ch-Ua-Platform":         "HTTP_SEC_CH_UA_PLATFORM",
	"Sec-Ch-Ua-Arch":             "HTTP_SEC_CH_UA_ARCH",
	"Sec-Ch-Ua-Full-Version":     "HTTP_SEC_CH_UA_FULL_VERSION",
	"Sec-Ch-Ua-Platform-Version": "HTTP_SEC_CH_UA_PLATFORM_VERSION",
	"Sec-Ch-Ua-Model":            "HTTP_SEC_CH_UA_MODEL",
	"Sec-GPC":                    "HTTP_SEC_GPC",
	// Reverse proxy headers
	"Forwarded":                         "HTTP_FORWARDED",
	"Via":                               "HTTP_VIA",
	"X-Requested-With":                  "HTTP_X_REQUESTED_WITH",
	"X-Forwarded-For":                   "HTTP_X_FORWARDED_FOR",
	"X-Forwarded-Host":                  "HTTP_X_FORWARDED_HOST",
	"X-Forwarded-Port":                  "HTTP_X_FORWARDED_PORT",
	"X-Forwarded-Proto":                 "HTTP_X_FORWARDED_PROTO",
	"X-Scheme":                          "HTTP_X_SCHEME",
	"X-Request-ID":                      "HTTP_X_REQUEST_ID",
	"X-Accel-Internal":                  "HTTP_X_ACCEL_INTERNAL",
	"X-Accel-Redirect":                  "HTTP_X_ACCEL_REDIRECT",
	"X-Real-IP":                         "HTTP_X_REAL_IP",
	"X-Frame-Options":                   "HTTP_X_FRAME_OPTIONS",
	"X-Content-Type-Options":            "HTTP_X_CONTENT_TYPE_OPTIONS",
	"X-XSS-Protection":                  "HTTP_X_XSS_PROTECTION",
	"X-Permitted-Cross-Domain-Policies": "HTTP_X_PERMITTED_CROSS_DOMAIN_POLICIES",
	"Front-End-Https":                   "HTTP_FRONT_END_HTTPS",
	"Proxy-Authorization":               "HTTP_PROXY_AUTHORIZATION",
	// Cloudflare/Cloudfront/Google Cloud headers
	"Cloudflare-Visitor":        "HTTP_CLOUDFLARE_VISITOR",
	"Cloudfront-Viewer-Address": "HTTP_CLOUDFRONT_VIEWER_ADDRESS",
	"Cloudfront-Viewer-Country": "HTTP_CLOUDFRONT_VIEWER_COUNTRY",
	"X-Amzn-Trace-Id":           "HTTP_X_AMZN_TRACE_ID",
	"X-Cloud-Trace-Context":     "HTTP_X_CLOUD_TRACE_CONTEXT",
	"CF-Ray":                    "HTTP_CF_RAY",
	"CF-Visitor":                "HTTP_CF_VISITOR",
	"CF-Request-ID":             "HTTP_CF_REQUEST_ID",
	"CF-IPCountry":              "HTTP_CF_IPCOUNTRY",
	"X-Device-Type":             "HTTP_X_DEVICE_TYPE",
	"X-Network-Info":            "HTTP_X_NETWORK_INFO",
	"X-Correlation-ID":          "HTTP_X_CORRELATION_ID",
	"X-Client-ID":               "HTTP_X_CLIENT_ID",
	"X-Debug-Info":              "HTTP_X_DEBUG_INFO",
	// Other headers
	"Accept-Patch":        "HTTP_ACCEPT_PATCH",
	"Accept-Ranges":       "HTTP_ACCEPT_RANGES",
	"Timing-Allow-Origin": "HTTP_TIMING_ALLOW_ORIGIN",
	"Expect":              "HTTP_EXPECT",
	"Alt-Svc":             "HTTP_ALT_SVC",
	"Early-Data":          "HTTP_EARLY_DATA",
	"Warning":             "HTTP_WARNING",
	"Priority":            "HTTP_PRIORITY",
	"TE":                  "HTTP_TE",
	"Trailer":             "HTTP_TRAILER",
	"Range":               "HTTP_RANGE",
	"Clear-Site-Data":     "HTTP_CLEAR_SITE_DATA",
	"Etag":                "HTTP_ETAG",
}

func GetCommonHeader(key string) string {
	return commonRequestHeaders[key]
}

func GetUnCommonHeader(key string) string {
	return "HTTP_" + headerNameReplacer.Replace(strings.ToUpper(key)) + "\x00"
}
