package headers

import (
	"strings"
)

// Translate header names to PHP header names
// All headers in 'commonHeaders' can be cached and registered safely
// All other headers must be prefixed with 'HTTP_' and sanitized
var headerNameReplacer = strings.NewReplacer(" ", "_", "-", "_")

// see also: https://en.wikipedia.org/wiki/List_of_HTTP_header_fields#Standard_request_fields
var commonRequestHeaders = map[string]string{
	// headers start
	"Accept":                            "HTTP_ACCEPT",
	"Accept-Charset":                    "HTTP_ACCEPT_CHARSET",
	"Accept-Encoding":                   "HTTP_ACCEPT_ENCODING",
	"Accept-Language":                   "HTTP_ACCEPT_LANGUAGE",
	"Access-Control-Request-Headers":    "HTTP_ACCESS_CONTROL_REQUEST_HEADERS",
	"Access-Control-Request-Method":     "HTTP_ACCESS_CONTROL_REQUEST_METHOD",
	"Authorization":                     "HTTP_AUTHORIZATION",
	"Cache-Control":                     "HTTP_CACHE_CONTROL",
	"Connection":                        "HTTP_CONNECTION",
	"Content-Disposition":               "HTTP_CONTENT_DISPOSITION",
	"Content-Encoding":                  "HTTP_CONTENT_ENCODING",
	"Content-Length":                    "HTTP_CONTENT_LENGTH",
	"Content-Type":                      "HTTP_CONTENT_TYPE",
	"Cookie":                            "HTTP_COOKIE",
	"Date":                              "HTTP_DATE",
	"Device-Memory":                     "HTTP_DEVICE_MEMORY",
	"DNT":                               "HTTP_DNT",
	"Downlink":                          "HTTP_DOWNLINK",
	"DPR":                               "HTTP_DPR",
	"Early-Data":                        "HTTP_EARLY_DATA",
	"ECT":                               "HTTP_ECT",
	"AM-I":                              "HTTP_AM_I",
	"Expect":                            "HTTP_EXPECT",
	"Forwarded":                         "HTTP_FORWARDED",
	"From":                              "HTTP_FROM",
	"Host":                              "HTTP_HOST",
	"If-Match":                          "HTTP_IF_MATCH",
	"If-Modified-Since":                 "HTTP_IF_MODIFIED_SINCE",
	"If-None-Match":                     "HTTP_IF_NONE_MATCH",
	"If-Range":                          "HTTP_IF_RANGE",
	"If-Unmodified-Since":               "HTTP_IF_UNMODIFIED_SINCE",
	"Keep-Alive":                        "HTTP_KEEP_ALIVE",
	"Max-Forwards":                      "HTTP_MAX_FORWARDS",
	"Origin":                            "HTTP_ORIGIN",
	"Pragma":                            "HTTP_PRAGMA",
	"Proxy-Authorization":               "HTTP_PROXY_AUTHORIZATION",
	"Range":                             "HTTP_RANGE",
	"Referer":                           "HTTP_REFERER",
	"RTT":                               "HTTP_RTT",
	"Save-Data":                         "HTTP_SAVE_DATA",
	"Sec-CH-UA":                         "HTTP_SEC_CH_UA",
	"Sec-CH-UA-Arch":                    "HTTP_SEC_CH_UA_ARCH",
	"Sec-CH-UA-Bitness":                 "HTTP_SEC_CH_UA_BITNESS",
	"Sec-CH-UA-Full-Version":            "HTTP_SEC_CH_UA_FULL_VERSION",
	"Sec-CH-UA-Full-Version-List":       "HTTP_SEC_CH_UA_FULL_VERSION_LIST",
	"Sec-CH-UA-Mobile":                  "HTTP_SEC_CH_UA_MOBILE",
	"Sec-CH-UA-Model":                   "HTTP_SEC_CH_UA_MODEL",
	"Sec-CH-UA-Platform":                "HTTP_SEC_CH_UA_PLATFORM",
	"Sec-CH-UA-Platform-Version":        "HTTP_SEC_CH_UA_PLATFORM_VERSION",
	"Sec-Fetch-Dest":                    "HTTP_SEC_FETCH_DEST",
	"Sec-Fetch-Mode":                    "HTTP_SEC_FETCH_MODE",
	"Sec-Fetch-Site":                    "HTTP_SEC_FETCH_SITE",
	"Sec-Fetch-User":                    "HTTP_SEC_FETCH_USER",
	"Sec-GPC":                           "HTTP_SEC_GPC",
	"Service-Worker-Navigation-Preload": "HTTP_SERVICE_WORKER_NAVIGATION_PRELOAD",
	"TE":                                "HTTP_TE",
	"Trailer":                           "HTTP_TRAILER",
	"Transfer-Encoding":                 "HTTP_TRANSFER_ENCODING",
	"Upgrade":                           "HTTP_UPGRADE",
	"Upgrade-Insecure-Requests":         "HTTP_UPGRADE_INSECURE_REQUESTS",
	"User-Agent":                        "HTTP_USER_AGENT",
	"Via":                               "HTTP_VIA",
	"Viewport-Width":                    "HTTP_VIEWPORT_WIDTH",
	"Want-Digest":                       "HTTP_WANT_DIGEST",
	"Warning":                           "HTTP_WARNING",
	"Width":                             "HTTP_WIDTH",
	"X-Forwarded-For":                   "HTTP_X_FORWARDED_FOR",
	"X-Forwarded-Host":                  "HTTP_X_FORWARDED_HOST",
	"X-Forwarded-Proto":                 "HTTP_X_FORWARDED_PROTO",
	"A-IM":                              "HTTP_A_IM",
	"Accept-Datetime":                   "HTTP_ACCEPT_DATETIME",
	"Content-MD5":                       "HTTP_CONTENT_MD5",
	"HTTP2-Settings":                    "HTTP_HTTP2_SETTINGS",
	"Prefer":                            "HTTP_PREFER",
	"X-Requested-With":                  "HTTP_X_REQUESTED_WITH",
	"Front-End-Https":                   "HTTP_FRONT_END_HTTPS",
	"X-Http-Method-Override":            "HTTP_X_HTTP_METHOD_OVERRIDE",
	"X-ATT-DeviceId":                    "HTTP_X_ATT_DEVICEID",
	"X-Wap-Profile":                     "HTTP_X_WAP_PROFILE",
	"Proxy-Connection":                  "HTTP_PROXY_CONNECTION",
	"X-UIDH":                            "HTTP_X_UIDH",
	"X-Csrf-Token":                      "HTTP_X_CSRF_TOKEN",
	"X-Request-ID":                      "HTTP_X_REQUEST_ID",
	"X-Correlation-ID":                  "HTTP_X_CORRELATION_ID",
	// headers end

	// common 3rd party headers
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
	"X-Client-ID":               "HTTP_X_CLIENT_ID",
	// header from #1181
	"X-Livewire": "HTTP_X_LIVEWIRE",
}

func GetCommonHeader(key string) string {
	return commonRequestHeaders[key]
}

func GetUnCommonHeader(key string) string {
	return "HTTP_" + headerNameReplacer.Replace(strings.ToUpper(key)) + "\x00"
}
