package frankenphp

import (
	"context"
	"net/http"
	"os"
	"strings"
	"time"

	"go.uber.org/zap"
)

// frankenPHPContext provides contextual information about the Request to handle.
type frankenPHPContext struct {
	documentRoot    string
	splitPath       []string
	env             PreparedEnv
	logger          *zap.Logger
	request         *http.Request
	originalRequest *http.Request

	docURI         string
	pathInfo       string
	scriptName     string
	scriptFilename string

	// Whether the request is already closed by us
	isDone bool

	responseWriter http.ResponseWriter
	exitStatus     int

	done      chan interface{}
	startedAt time.Time
}

// fromContext extracts the frankenPHPContext from a context.
func fromContext(ctx context.Context) (fctx *frankenPHPContext, ok bool) {
	fctx, ok = ctx.Value(contextKey).(*frankenPHPContext)
	return
}

// NewRequestWithContext creates a new FrankenPHP request context.
func NewRequestWithContext(r *http.Request, opts ...RequestOption) (*http.Request, error) {
	fc := &frankenPHPContext{
		done:      make(chan interface{}),
		startedAt: time.Now(),
		request:   r,
	}
	for _, o := range opts {
		if err := o(fc); err != nil {
			return nil, err
		}
	}

	if fc.logger == nil {
		fc.logger = logger
	}

	if fc.documentRoot == "" {
		if EmbeddedAppPath != "" {
			fc.documentRoot = EmbeddedAppPath
		} else {
			var err error
			if fc.documentRoot, err = os.Getwd(); err != nil {
				return nil, err
			}
		}
	}

	if fc.splitPath == nil {
		fc.splitPath = []string{".php"}
	}

	if fc.env == nil {
		fc.env = make(map[string]string)
	}

	if splitPos := splitPos(fc, r.URL.Path); splitPos > -1 {
		fc.docURI = r.URL.Path[:splitPos]
		fc.pathInfo = r.URL.Path[splitPos:]

		// Strip PATH_INFO from SCRIPT_NAME
		fc.scriptName = strings.TrimSuffix(r.URL.Path, fc.pathInfo)

		// Ensure the SCRIPT_NAME has a leading slash for compliance with RFC3875
		// Info: https://tools.ietf.org/html/rfc3875#section-4.1.13
		if fc.scriptName != "" && !strings.HasPrefix(fc.scriptName, "/") {
			fc.scriptName = "/" + fc.scriptName
		}
	}

	// SCRIPT_FILENAME is the absolute path of SCRIPT_NAME
	fc.scriptFilename = sanitizedPathJoin(fc.documentRoot, fc.scriptName)

	c := context.WithValue(r.Context(), contextKey, fc)

	return r.WithContext(c), nil
}

func newFakeContext(requestPath string, opts ...RequestOption) (*frankenPHPContext, error) {
	r, err := http.NewRequest(http.MethodGet, requestPath, nil)
	if err != nil {
		return nil, err
	}

	fr, err := NewRequestWithContext(r, opts...)
	if err != nil {
		return nil, err
	}

	fc, _ := fromContext(fr.Context())

	return fc, nil
}

// closeContext sends the response to the client
func (fc *frankenPHPContext) closeContext() {
	if fc.isDone {
		return
	}

	close(fc.done)
	fc.isDone = true
}

// validate checks if the request should be outright rejected
func (fc *frankenPHPContext) validate() bool {
	if !strings.Contains(fc.request.URL.Path, "\x00") {
		return true
	}

	fc.rejectBadRequest("Invalid request path")

	return false
}

func (fc *frankenPHPContext) clientHasClosed() bool {
	select {
	case <-fc.request.Context().Done():
		return true
	default:
		return false
	}
}

// reject sends a response with the given status code and message
func (fc *frankenPHPContext) reject(statusCode int, message string) {
	if fc.isDone {
		return
	}

	rw := fc.responseWriter
	if rw != nil {
		rw.WriteHeader(statusCode)
		_, _ = rw.Write([]byte(message))
		rw.(http.Flusher).Flush()
	}

	fc.closeContext()
}

func (fc *frankenPHPContext) rejectBadRequest(message string) {
	fc.reject(http.StatusBadRequest, message)
}
