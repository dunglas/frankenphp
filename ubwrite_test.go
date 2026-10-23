package frankenphp

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"testing"
	"unsafe"
)

func TestUBWrite(t *testing.T) {
	for _, data := range []string{"", "a", "worker startup", "a\x00b\xff", strings.Repeat("x", 1024)} {
		for _, mode := range []string{"log", "disabled", "http", "error", "short", "done", "done_closed", "canceled"} {
			t.Run(mode+"/"+data, func(t *testing.T) { checkUBWrite(t, data, mode) })
		}
	}
}

func FuzzUBWrite(f *testing.F) {
	f.Add("worker startup", uint8(0))
	f.Add("a\x00b\xff", uint8(1))
	f.Add("", uint8(2))
	f.Fuzz(func(t *testing.T, data string, mode uint8) {
		modes := []string{"log", "disabled", "http", "error", "short", "done", "done_closed", "canceled"}
		checkUBWrite(t, data, modes[int(mode)%len(modes)])
	})
}

func checkUBWrite(t *testing.T, message, mode string) {
	t.Helper()
	data := []byte(message)
	capture := &ubWriteCapture{level: slog.LevelInfo}
	fc := &frankenPHPContext{ctx: context.Background(), logger: slog.New(capture)}
	writer := &ubWriteResponseWriter{n: -1}
	expectedN, expectedClosed := len(data), false
	switch mode {
	case "disabled":
		capture.level = slog.LevelWarn
	case "http":
		fc.responseWriter = writer
	case "error":
		writer.n = 0
		writer.err = io.ErrClosedPipe
		fc.responseWriter = writer
		capture.level = slog.LevelDebug
		expectedN = 0
	case "short":
		writer.n = 0
		fc.responseWriter = writer
		expectedN = 0
	case "done":
		fc.isDone = true
	case "done_closed":
		fc.isDone = true
		fc.clientHadClosed = true
		expectedClosed = true
	case "canceled":
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		fc.ctx = ctx
		fc.request = &http.Request{}
		expectedClosed = true
	}
	oldThreads := phpThreads
	phpThreads = []*phpThread{{handler: &regularThread{fc: fc}}}
	t.Cleanup(func() { phpThreads = oldThreads })
	n, closed := invokeUBWrite(data)
	if n != expectedN || closed != expectedClosed {
		t.Fatalf("write = (%d,%v), want (%d,%v)", n, closed, expectedN, expectedClosed)
	}
	// A handler is allowed to retain the record after the source buffer changes.
	for i := range data {
		data[i] = 'Z'
	}
	records := capture.snapshot()
	switch mode {
	case "log", "canceled":
		if len(records) != 1 || records[0].Message != message || records[0].Level != slog.LevelInfo {
			t.Fatalf("unexpected log records: %v", records)
		}
	case "error":
		if len(records) != 1 || records[0].Message != "write error" || records[0].Level != slog.LevelDebug {
			t.Fatalf("unexpected error records: %v", records)
		}
		var loggedError error
		records[0].Attrs(func(a slog.Attr) bool {
			if a.Key == "error" {
				loggedError, _ = a.Value.Any().(error)
			}
			return true
		})
		if loggedError != io.ErrClosedPipe {
			t.Fatalf("logged error = %v", loggedError)
		}
	default:
		if len(records) != 0 {
			t.Fatalf("unexpected log records: %v", records)
		}
	}
	if mode == "http" || mode == "error" || mode == "short" {
		if string(writer.data) != message {
			t.Fatalf("response bytes = %q, want %q", writer.data, message)
		}
	}
}

// Reflection bridges the C argument types without adding production test hooks
// or importing C from a _test.go file (which Go does not support).
func invokeUBWrite(data []byte) (int, bool) {
	fn := reflect.ValueOf(go_ub_write)
	signature := fn.Type()
	values := fn.Call([]reflect.Value{
		reflect.Zero(signature.In(0)),
		reflect.NewAt(signature.In(1).Elem(), unsafe.Pointer(unsafe.SliceData(data))),
		reflect.ValueOf(len(data)).Convert(signature.In(2)),
	})
	return int(values[0].Uint()), values[1].Bool()
}

type ubWriteCapture struct {
	level   slog.Level
	mu      sync.Mutex
	records []slog.Record
}

func (h *ubWriteCapture) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level
}
func (h *ubWriteCapture) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.records = append(h.records, r.Clone())
	return nil
}
func (h *ubWriteCapture) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h *ubWriteCapture) WithGroup(string) slog.Handler      { return h }
func (h *ubWriteCapture) snapshot() []slog.Record {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]slog.Record(nil), h.records...)
}

type ubWriteResponseWriter struct {
	data []byte
	n    int
	err  error
}

func (w *ubWriteResponseWriter) Header() http.Header { return nil }
func (w *ubWriteResponseWriter) WriteHeader(int)     {}
func (w *ubWriteResponseWriter) Write(p []byte) (int, error) {
	w.data = append(w.data, p...)
	if w.n >= 0 {
		return w.n, w.err
	}
	return len(p), w.err
}
