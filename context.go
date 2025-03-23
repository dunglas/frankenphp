package frankenphp

import (
	"net/http"
	"os"
	"strings"
	"time"

	"go.uber.org/zap"
)

// FrankenPHPContext provides contextual information about the Request to handle.
type FrankenPHPContext struct {
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

	done      chan interface{}
	startedAt time.Time
}

// NewRequestWithContext creates a new FrankenPHP request context.
func NewRequestWithContext(r *http.Request, opts ...RequestOption) (*FrankenPHPContext, error) {
	fc := &FrankenPHPContext{
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

	return fc, nil
}

func newDummyContext(requestPath string, opts ...RequestOption) (*FrankenPHPContext, error) {
	r, err := http.NewRequest(http.MethodGet, requestPath, nil)
	if err != nil {
		return nil, err
	}

	fc, err := NewRequestWithContext(r, opts...)
	if err != nil {
		return nil, err
	}

	return fc, nil
}

// closeContext sends the response to the client
func (fc *FrankenPHPContext) closeContext() {
	if fc.isDone {
		return
	}

	close(fc.done)
	fc.isDone = true
}

// validate checks if the request should be outright rejected
func (fc *FrankenPHPContext) validate() bool {
	if !strings.Contains(fc.request.URL.Path, "\x00") {
		return true
	}

	fc.rejectBadRequest("Invalid request path")

	return false
}

func (fc *FrankenPHPContext) clientHasClosed() bool {
	select {
	case <-fc.request.Context().Done():
		return true
	default:
		return false
	}
}

// reject sends a response with the given status code and message
func (fc *FrankenPHPContext) reject(statusCode int, message string) {
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

func (fc *FrankenPHPContext) rejectBadRequest(message string) {
	fc.reject(http.StatusBadRequest, message)
}
