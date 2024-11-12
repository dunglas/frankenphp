package frankenphp

import (
	"github.com/dunglas/frankenphp/internal/fastabs"
	"regexp"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

var metricsNameRegex = regexp.MustCompile(`\W+`)
var metricsNameFixRegex = regexp.MustCompile(`^_+|_+$`)

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
	// TotalWorkers collects expected workers
	TotalWorkers(name string, num int)
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

func (n nullMetrics) TotalWorkers(string, int) {
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
	totalWorkers       map[string]prometheus.Gauge
	busyWorkers        map[string]prometheus.Gauge
	readyWorkers       map[string]prometheus.Gauge
	workerCrashes      map[string]prometheus.Counter
	workerRestarts     map[string]prometheus.Counter
	workerRequestTime  map[string]prometheus.Counter
	workerRequestCount map[string]prometheus.Counter
	mu                 sync.Mutex
}

func (m *PrometheusMetrics) StartWorker(name string) {
	m.busyThreads.Inc()

	// tests do not register workers before starting them
	if _, ok := m.totalWorkers[name]; !ok {
		return
	}
	m.totalWorkers[name].Inc()
}

func (m *PrometheusMetrics) ReadyWorker(name string) {
	if _, ok := m.totalWorkers[name]; !ok {
		return
	}

	m.readyWorkers[name].Inc()
}

func (m *PrometheusMetrics) StopWorker(name string, reason StopReason) {
	m.busyThreads.Dec()

	// tests do not register workers before starting them
	if _, ok := m.totalWorkers[name]; !ok {
		return
	}
	m.totalWorkers[name].Dec()
	m.readyWorkers[name].Dec()

	if reason == StopReasonCrash {
		m.workerCrashes[name].Inc()
	} else if reason == StopReasonRestart {
		m.workerRestarts[name].Inc()
	} else if reason == StopReasonShutdown {
		m.totalWorkers[name].Dec()
	}
}

func (m *PrometheusMetrics) getIdentity(name string) (string, error) {
	actualName, err := fastabs.FastAbs(name)
	if err != nil {
		return name, err
	}

	return actualName, nil
}

func (m *PrometheusMetrics) TotalWorkers(name string, _ int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	identity, err := m.getIdentity(name)
	if err != nil {
		// do not create metrics, let error propagate when worker is started
		return
	}

	subsystem := getWorkerNameForMetrics(name)

	if _, ok := m.totalWorkers[identity]; !ok {
		m.totalWorkers[identity] = prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "frankenphp",
			Subsystem: subsystem,
			Name:      "total_workers",
			Help:      "Total number of PHP workers for this worker",
		})
		m.registry.MustRegister(m.totalWorkers[identity])
	}

	if _, ok := m.workerCrashes[identity]; !ok {
		m.workerCrashes[identity] = prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "frankenphp",
			Subsystem: subsystem,
			Name:      "worker_crashes",
			Help:      "Number of PHP worker crashes for this worker",
		})
		m.registry.MustRegister(m.workerCrashes[identity])
	}

	if _, ok := m.workerRestarts[identity]; !ok {
		m.workerRestarts[identity] = prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "frankenphp",
			Subsystem: subsystem,
			Name:      "worker_restarts",
			Help:      "Number of PHP worker restarts for this worker",
		})
		m.registry.MustRegister(m.workerRestarts[identity])
	}

	if _, ok := m.readyWorkers[identity]; !ok {
		m.readyWorkers[identity] = prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "frankenphp",
			Subsystem: subsystem,
			Name:      "ready_workers",
			Help:      "Running workers that have successfully called frankenphp_handle_request at least once",
		})
		m.registry.MustRegister(m.readyWorkers[identity])
	}

	if _, ok := m.busyWorkers[identity]; !ok {
		m.busyWorkers[identity] = prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "frankenphp",
			Subsystem: subsystem,
			Name:      "busy_workers",
			Help:      "Number of busy PHP workers for this worker",
		})
		m.registry.MustRegister(m.busyWorkers[identity])
	}

	if _, ok := m.workerRequestTime[identity]; !ok {
		m.workerRequestTime[identity] = prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "frankenphp",
			Subsystem: subsystem,
			Name:      "worker_request_time",
		})
		m.registry.MustRegister(m.workerRequestTime[identity])
	}

	if _, ok := m.workerRequestCount[identity]; !ok {
		m.workerRequestCount[identity] = prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "frankenphp",
			Subsystem: subsystem,
			Name:      "worker_request_count",
		})
		m.registry.MustRegister(m.workerRequestCount[identity])
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
	if _, ok := m.workerRequestTime[name]; !ok {
		return
	}

	m.workerRequestCount[name].Inc()
	m.busyWorkers[name].Dec()
	m.workerRequestTime[name].Add(duration.Seconds())
}

func (m *PrometheusMetrics) StartWorkerRequest(name string) {
	if _, ok := m.busyWorkers[name]; !ok {
		return
	}
	m.busyWorkers[name].Inc()
}

func (m *PrometheusMetrics) Shutdown() {
	m.registry.Unregister(m.totalThreads)
	m.registry.Unregister(m.busyThreads)

	for _, g := range m.totalWorkers {
		m.registry.Unregister(g)
	}

	for _, g := range m.busyWorkers {
		m.registry.Unregister(g)
	}

	for _, c := range m.workerRequestTime {
		m.registry.Unregister(c)
	}

	for _, c := range m.workerRequestCount {
		m.registry.Unregister(c)
	}

	for _, c := range m.workerCrashes {
		m.registry.Unregister(c)
	}

	for _, c := range m.workerRestarts {
		m.registry.Unregister(c)
	}

	for _, g := range m.readyWorkers {
		m.registry.Unregister(g)
	}

	m.totalThreads = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "frankenphp_total_threads",
		Help: "Total number of PHP threads",
	})
	m.busyThreads = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "frankenphp_busy_threads",
		Help: "Number of busy PHP threads",
	})
	m.totalWorkers = map[string]prometheus.Gauge{}
	m.busyWorkers = map[string]prometheus.Gauge{}
	m.workerRequestTime = map[string]prometheus.Counter{}
	m.workerRequestCount = map[string]prometheus.Counter{}
	m.workerRestarts = map[string]prometheus.Counter{}
	m.workerCrashes = map[string]prometheus.Counter{}
	m.readyWorkers = map[string]prometheus.Gauge{}

	m.registry.MustRegister(m.totalThreads)
	m.registry.MustRegister(m.busyThreads)
}

func getWorkerNameForMetrics(name string) string {
	name = metricsNameRegex.ReplaceAllString(name, "_")
	name = metricsNameFixRegex.ReplaceAllString(name, "")

	return name
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
		totalWorkers:       map[string]prometheus.Gauge{},
		busyWorkers:        map[string]prometheus.Gauge{},
		workerRequestTime:  map[string]prometheus.Counter{},
		workerRequestCount: map[string]prometheus.Counter{},
		workerRestarts:     map[string]prometheus.Counter{},
		workerCrashes:      map[string]prometheus.Counter{},
		readyWorkers:       map[string]prometheus.Gauge{},
	}

	m.registry.MustRegister(m.totalThreads)
	m.registry.MustRegister(m.busyThreads)

	return m
}
