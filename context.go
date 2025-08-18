package frankenphp

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// frankenPHPContext provides contextual information about the Request to handle.
type frankenPHPContext struct {
	documentRoot    string
	splitPath       []string
	env             PreparedEnv
	logger          *slog.Logger
	request         *http.Request
	originalRequest *http.Request
	worker          *worker

	docURI         string
	pathInfo       string
	scriptName     string
	scriptFilename string

	// Whether the request is already closed by us
	isDone bool

	responseWriter http.ResponseWriter

	done      chan any
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
		done:      make(chan any),
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

	// if a worker is already assigned explicitly, use its filename and skip parsing path variables
	if fc.worker != nil {
		fc.scriptFilename = fc.worker.fileName
	} else {
		// if no worker was assigned, split the path into the 'traditional' CGI path variables
		// this needs to already happen here in case a worker script still matches the path
		splitCgiPath(fc)
	}

	c := context.WithValue(r.Context(), contextKey, fc)

	return r.WithContext(c), nil
}

// newDummyContext creates a fake context from a request path.
func newDummyContext(requestPath string, opts ...RequestOption) (*frankenPHPContext, error) {
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
	if strings.Contains(fc.request.URL.Path, "\x00") {
		fc.rejectBadRequest("Invalid request path")
		return false
	}

	// TODO: Caddy already does this, probably unnecessary
	contentLengthStr := fc.request.Header.Get("Content-Length")
	if contentLengthStr != "" {
		contentLength, err := strconv.Atoi(contentLengthStr)
		if err != nil || contentLength < 0 {
			fc.rejectBadRequest("invalid Content-Length header: " + contentLengthStr)

			return false
		}
	}

	return true
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

		if f, ok := rw.(http.Flusher); ok {
			f.Flush()
		}
	}

	fc.closeContext()
}

func (fc *frankenPHPContext) rejectBadRequest(message string) {
	fc.reject(http.StatusBadRequest, message)
}
