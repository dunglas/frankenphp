package frankenphp

import (
	"github.com/prometheus/client_golang/prometheus"
	"regexp"
	"sync"
	"time"
)

var metricsNameRegex = regexp.MustCompile(`\W+`)
var metricsNameFixRegex = regexp.MustCompile(`^_+|_+$`)

type Metrics interface {
	// StartWorker Collects started workers
	StartWorker(name string)
	// StopWorker Collects stopped workers
	StopWorker(name string)
	// TotalWorkers Collects expected workers
	TotalWorkers(name string, num int)
	// TotalThreads Collects total threads
	TotalThreads(num int)
	// StartRequest Collects started requests
	StartRequest()
	// StopRequest Collects stopped requests
	StopRequest()
	// StopWorkerRequest Collects stopped worker requests
	StopWorkerRequest(name string, duration time.Duration)
	StartWorkerRequest(name string)
	RenameWorker(oldName, newName string)
	Shutdown()
}

type nullMetrics struct{}

func (n nullMetrics) StartWorker(name string) {
}

func (n nullMetrics) StopWorker(name string) {
}

func (n nullMetrics) TotalWorkers(name string, num int) {
}

func (n nullMetrics) TotalThreads(num int) {
}

func (n nullMetrics) StartRequest() {
}

func (n nullMetrics) StopRequest() {
}

func (n nullMetrics) StopWorkerRequest(name string, duration time.Duration) {
}

func (n nullMetrics) StartWorkerRequest(name string) {
}

func (n nullMetrics) RenameWorker(oldName, newName string) {
}

func (n nullMetrics) Shutdown() {
}

type PrometheusMetrics struct {
	registry           prometheus.Registerer
	totalThreads       prometheus.Counter
	busyThreads        prometheus.Gauge
	totalWorkers       map[string]prometheus.Gauge
	busyWorkers        map[string]prometheus.Gauge
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

func (m *PrometheusMetrics) StopWorker(name string) {
	m.busyThreads.Dec()

	// tests do not register workers before starting them
	if _, ok := m.totalWorkers[name]; !ok {
		return
	}
	m.totalWorkers[name].Dec()
}

func (m *PrometheusMetrics) TotalWorkers(name string, num int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	subsystem := getWorkerNameForMetrics(name)

	if _, ok := m.totalWorkers[name]; !ok {
		m.totalWorkers[name] = prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "frankenphp",
			Subsystem: subsystem,
			Name:      "total_workers",
			Help:      "Total number of PHP workers for this worker",
		})
		m.registry.MustRegister(m.totalWorkers[name])
	}

	if _, ok := m.busyWorkers[name]; !ok {
		m.busyWorkers[name] = prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "frankenphp",
			Subsystem: subsystem,
			Name:      "busy_workers",
			Help:      "Number of busy PHP workers for this worker",
		})
		m.registry.MustRegister(m.busyWorkers[name])
	}

	if _, ok := m.workerRequestTime[name]; !ok {
		m.workerRequestTime[name] = prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "frankenphp",
			Subsystem: subsystem,
			Name:      "worker_request_time",
		})
		m.registry.MustRegister(m.workerRequestTime[name])
	}

	if _, ok := m.workerRequestCount[name]; !ok {
		m.workerRequestCount[name] = prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "frankenphp",
			Subsystem: subsystem,
			Name:      "worker_request_count",
		})
		m.registry.MustRegister(m.workerRequestCount[name])
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

func (m *PrometheusMetrics) RenameWorker(oldName, newName string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.totalWorkers[oldName]; ok {
		m.totalWorkers[newName] = m.totalWorkers[oldName]
		delete(m.totalWorkers, oldName)
	}

	if _, ok := m.busyWorkers[oldName]; ok {
		m.busyWorkers[newName] = m.busyWorkers[oldName]
		delete(m.busyWorkers, oldName)
	}

	if _, ok := m.workerRequestTime[oldName]; ok {
		m.workerRequestTime[newName] = m.workerRequestTime[oldName]
		delete(m.workerRequestTime, oldName)
	}

	if _, ok := m.workerRequestCount[oldName]; ok {
		m.workerRequestCount[newName] = m.workerRequestCount[oldName]
		delete(m.workerRequestCount, oldName)
	}
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
	}

	m.registry.MustRegister(m.totalThreads)
	m.registry.MustRegister(m.busyThreads)

	return m
}
