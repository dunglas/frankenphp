package frankenphp

import (
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const (
	StopReasonCrash = iota
	StopReasonRestart
	StopReasonShutdown
)

type StopReason int

type Metrics interface {
	// StartWorker collects started workers
	StartWorker(name string)
	// ReadyWorker collects ready workers
	ReadyWorker(name string)
	// StopWorker collects stopped workers
	StopWorker(name string, reason StopReason)
	// TotalThreads collects total threads
	TotalThreads(num int)
	// StartRequest collects started requests
	StartRequest()
	// StopRequest collects stopped requests
	StopRequest()
	// StopWorkerRequest collects stopped worker requests
	StopWorkerRequest(name string, duration time.Duration)
	// StartWorkerRequest collects started worker requests
	StartWorkerRequest(name string)
	Shutdown()
}

type nullMetrics struct{}

func (n nullMetrics) StartWorker(string) {
}

func (n nullMetrics) ReadyWorker(string) {
}

func (n nullMetrics) StopWorker(string, StopReason) {
}

func (n nullMetrics) TotalThreads(int) {
}

func (n nullMetrics) StartRequest() {
}

func (n nullMetrics) StopRequest() {
}

func (n nullMetrics) StopWorkerRequest(string, time.Duration) {
}

func (n nullMetrics) StartWorkerRequest(string) {
}

func (n nullMetrics) Shutdown() {
}

type PrometheusMetrics struct {
	registry           prometheus.Registerer
	totalThreads       prometheus.Counter
	busyThreads        prometheus.Gauge
	totalWorkers       *prometheus.GaugeVec
	busyWorkers        *prometheus.GaugeVec
	readyWorkers       *prometheus.GaugeVec
	workerCrashes      *prometheus.CounterVec
	workerRestarts     *prometheus.CounterVec
	workerRequestTime  *prometheus.CounterVec
	workerRequestCount *prometheus.CounterVec
	mu                 sync.Mutex
}

func initWorkerMetrics(metrics Metrics) {
	prometheusMetrics, ok := metrics.(*PrometheusMetrics)
	if !ok {
		return
	}

	prometheusMetrics.mu.Lock()
	defer prometheusMetrics.mu.Unlock()

	const ns, sub = "frankenphp", "worker"
	basicLabels := []string{"worker"}

	prometheusMetrics.totalWorkers = promauto.With(prometheusMetrics.registry).NewGaugeVec(prometheus.GaugeOpts{
		Namespace: ns,
		Name:      "total_workers",
		Help:      "Total number of PHP workers for this worker",
	}, basicLabels)

	prometheusMetrics.readyWorkers = promauto.With(prometheusMetrics.registry).NewGaugeVec(prometheus.GaugeOpts{
		Namespace: ns,
		Name:      "ready_workers",
		Help:      "Running workers that have successfully called frankenphp_handle_request at least once",
	}, basicLabels)

	prometheusMetrics.busyWorkers = promauto.With(prometheusMetrics.registry).NewGaugeVec(prometheus.GaugeOpts{
		Namespace: ns,
		Name:      "busy_workers",
		Help:      "Number of busy PHP workers for this worker",
	}, basicLabels)

	prometheusMetrics.workerCrashes = promauto.With(prometheusMetrics.registry).NewCounterVec(prometheus.CounterOpts{
		Namespace: ns,
		Subsystem: sub,
		Name:      "crashes",
		Help:      "Number of PHP worker crashes for this worker",
	}, basicLabels)

	prometheusMetrics.workerRestarts = promauto.With(prometheusMetrics.registry).NewCounterVec(prometheus.CounterOpts{
		Namespace: ns,
		Subsystem: sub,
		Name:      "restarts",
		Help:      "Number of PHP worker restarts for this worker",
	}, basicLabels)

	prometheusMetrics.workerRequestTime = promauto.With(prometheusMetrics.registry).NewCounterVec(prometheus.CounterOpts{
		Namespace: ns,
		Subsystem: sub,
		Name:      "request_time",
	}, basicLabels)

	prometheusMetrics.workerRequestCount = promauto.With(prometheusMetrics.registry).NewCounterVec(prometheus.CounterOpts{
		Namespace: ns,
		Subsystem: sub,
		Name:      "request_count",
	}, basicLabels)
}

func (m *PrometheusMetrics) getLabels(name string) prometheus.Labels {
	return prometheus.Labels{"worker": name}
}

func (m *PrometheusMetrics) StartWorker(name string) {
	m.busyThreads.Inc()

	// tests do not register workers before starting them
	if m.totalWorkers == nil {
		return
	}

	m.totalWorkers.With(m.getLabels(name)).Inc()
}

func (m *PrometheusMetrics) ReadyWorker(name string) {
	if m.totalWorkers == nil {
		return
	}

	m.readyWorkers.With(m.getLabels(name)).Inc()
}

func (m *PrometheusMetrics) StopWorker(name string, reason StopReason) {
	m.busyThreads.Dec()

	// tests do not register workers before starting them
	if m.totalWorkers == nil {
		return
	}
	m.totalWorkers.With(m.getLabels(name)).Dec()
	m.readyWorkers.With(m.getLabels(name)).Dec()

	if reason == StopReasonCrash {
		m.workerCrashes.With(m.getLabels(name)).Inc()
	} else if reason == StopReasonRestart {
		m.workerRestarts.With(m.getLabels(name)).Inc()
	}
}

func (m *PrometheusMetrics) TotalThreads(num int) {
	m.totalThreads.Add(float64(num))
}

func (m *PrometheusMetrics) StartRequest() {
	m.busyThreads.Inc()
}

func (m *PrometheusMetrics) StopRequest() {
	m.busyThreads.Dec()
}

func (m *PrometheusMetrics) StopWorkerRequest(name string, duration time.Duration) {
	if m.workerRequestTime == nil {
		return
	}

	m.workerRequestCount.With(m.getLabels(name)).Inc()
	m.busyWorkers.With(m.getLabels(name)).Dec()
	m.workerRequestTime.With(m.getLabels(name)).Add(duration.Seconds())
}

func (m *PrometheusMetrics) StartWorkerRequest(name string) {
	if m.busyWorkers == nil {
		return
	}
	m.busyWorkers.With(m.getLabels(name)).Inc()
}

func (m *PrometheusMetrics) Shutdown() {
	m.registry.Unregister(m.totalThreads)
	m.registry.Unregister(m.busyThreads)

	m.registry.Unregister(m.totalWorkers)
	m.registry.Unregister(m.busyWorkers)
	m.registry.Unregister(m.workerRequestTime)
	m.registry.Unregister(m.workerRequestCount)
	m.registry.Unregister(m.workerCrashes)
	m.registry.Unregister(m.workerRestarts)
	m.registry.Unregister(m.readyWorkers)

	m.totalThreads = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "frankenphp_total_threads",
		Help: "Total number of PHP threads",
	})
	m.busyThreads = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "frankenphp_busy_threads",
		Help: "Number of busy PHP threads",
	})
	m.totalWorkers = nil
	m.busyWorkers = nil
	m.workerRequestTime = nil
	m.workerRequestCount = nil
	m.workerRestarts = nil
	m.workerCrashes = nil
	m.readyWorkers = nil

	m.registry.MustRegister(m.totalThreads)
	m.registry.MustRegister(m.busyThreads)
}

func NewPrometheusMetrics(registry prometheus.Registerer) *PrometheusMetrics {
	if registry == nil {
		registry = prometheus.NewRegistry()
	}

	m := &PrometheusMetrics{
		registry: registry,
		totalThreads: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "frankenphp_total_threads",
			Help: "Total number of PHP threads",
		}),
		busyThreads: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "frankenphp_busy_threads",
			Help: "Number of busy PHP threads",
		}),
		totalWorkers:       nil,
		busyWorkers:        nil,
		workerRequestTime:  nil,
		workerRequestCount: nil,
		workerRestarts:     nil,
		workerCrashes:      nil,
		readyWorkers:       nil,
	}

	m.registry.MustRegister(m.totalThreads)
	m.registry.MustRegister(m.busyThreads)

	return m
}
