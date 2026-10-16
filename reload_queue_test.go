package frankenphp

import (
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type reloadQueueMetrics struct {
	nullMetrics
	queued   chan struct{}
	started  atomic.Int32
	stopped  atomic.Int32
	dequeued atomic.Int32
}

func (m *reloadQueueMetrics) StartRequest()                           { m.started.Add(1) }
func (m *reloadQueueMetrics) StopRequest()                            { m.stopped.Add(1) }
func (m *reloadQueueMetrics) QueuedRequest()                          { close(m.queued) }
func (m *reloadQueueMetrics) DequeuedRequest()                        { m.dequeued.Add(1) }
func (m *reloadQueueMetrics) StartWorkerRequest(string)               { m.StartRequest() }
func (m *reloadQueueMetrics) StopWorkerRequest(string, time.Duration) { m.StopRequest() }
func (m *reloadQueueMetrics) QueuedWorkerRequest(string)              { m.QueuedRequest() }
func (m *reloadQueueMetrics) DequeuedWorkerRequest(string)            { m.DequeuedRequest() }

// A queued request has not run PHP yet. Retirement must return it to Caddy
// without writing a rejection or consuming the body, so it can be retried.
func TestReloadReturnsQueuedRequestsWithoutRejecting(t *testing.T) {
	regularThreadMu.Lock()
	savedThreads := regularThreads
	regularThreads = nil
	regularThreadMu.Unlock()
	savedScaleChan := scaleChan
	scaleChan = nil
	savedMaxWaitTime := maxWaitTime.Swap(0)
	t.Cleanup(func() {
		regularThreadMu.Lock()
		regularThreads = savedThreads
		regularThreadMu.Unlock()
		scaleChan = savedScaleChan
		maxWaitTime.Store(savedMaxWaitTime)
	})

	for _, kind := range []string{"regular", "worker"} {
		t.Run(kind, func(t *testing.T) {
			m := &reloadQueueMetrics{queued: make(chan struct{})}
			s := &Server{done: make(chan struct{}), metrics: m}
			s.isRegistered.Store(true)
			w := &worker{requestChan: make(chan *frankenPHPContext), metrics: m}
			r := httptest.NewRequest("POST", "http://example.com/index.php", strings.NewReader("preserved body"))
			response := httptest.NewRecorder()
			fc := &frankenPHPContext{
				request: r, responseWriter: response, ctx: r.Context(),
				server: s, done: make(chan any), startedAt: time.Now(),
			}
			result := make(chan error, 1)
			finished := make(chan struct{})
			var retired sync.Once
			retire := func() {
				retired.Do(func() {
					s.isRegistered.Store(false)
					close(s.done)
					if kind == "worker" {
						w.drainRequests()
					}
				})
			}
			t.Cleanup(func() {
				retire()
				select {
				case <-finished:
				case <-time.After(5 * time.Second):
					t.Error("queued request goroutine did not stop")
				}
			})
			go func() {
				defer close(finished)
				if kind == "worker" {
					result <- w.handleRequest(fc)
				} else {
					result <- handleRequestWithRegularPHPThreads(fc)
				}
			}()
			select {
			case <-m.queued:
			case <-time.After(5 * time.Second):
				t.Fatal("request never entered the queue")
			}
			retire()
			select {
			case err := <-result:
				require.ErrorIs(t, err, ErrNotRunning)
			case <-time.After(5 * time.Second):
				t.Fatal("retired request stayed queued")
			}
			require.Empty(t, response.Body.String())
			require.False(t, fc.isDone, "retirement must not reject or execute the request")
			body := make([]byte, len("preserved body"))
			_, err := r.Body.Read(body)
			require.NoError(t, err)
			require.Equal(t, "preserved body", string(body))
			require.EqualValues(t, 1, m.started.Load())
			require.EqualValues(t, 1, m.stopped.Load())
			require.EqualValues(t, 1, m.dequeued.Load())
			if kind == "worker" {
				require.EqualValues(t, -1<<31, w.queuedRequests.Load())
			} else {
				require.Zero(t, w.queuedRequests.Load())
			}
			require.Zero(t, queuedRegularThreads.Load())
		})
	}
}
