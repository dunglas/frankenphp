package frankenphp

import (
	"github.com/prometheus/client_golang/prometheus"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestGetWorkerNameForMetrics(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"worker-1", "worker_1"},
		{"worker@name", "worker_name"},
		{"worker name", "worker_name"},
		{"worker/name", "worker_name"},
		{"worker.name", "worker_name"},
		{"////worker////name...//worker", "worker_name_worker"},
	}

	for _, test := range tests {
		result := getWorkerNameForMetrics(test.input)
		assert.Equal(t, test.expected, result)
	}
}

func createPrometheusMetrics() *PrometheusMetrics {
	return &PrometheusMetrics{
		registry:           prometheus.NewRegistry(),
		totalThreads:       prometheus.NewCounter(prometheus.CounterOpts{Name: "total_threads"}),
		busyThreads:        prometheus.NewGauge(prometheus.GaugeOpts{Name: "busy_threads"}),
		totalWorkers:       make(map[string]prometheus.Gauge),
		busyWorkers:        make(map[string]prometheus.Gauge),
		workerRequestTime:  make(map[string]prometheus.Counter),
		workerRequestCount: make(map[string]prometheus.Counter),
		mu:                 sync.Mutex{},
	}
}

func TestPrometheusMetrics_TotalWorkers(t *testing.T) {
	m := createPrometheusMetrics()

	tests := []struct {
		name   string
		worker string
		num    int
	}{
		{"SetWorkers", "test_worker", 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m.TotalWorkers(tt.worker, tt.num)
			if _, ok := m.totalWorkers[tt.worker]; !ok {
				t.Fatalf("expected totalWorkers to be registered")
			}
		})
	}
}

func TestPrometheusMetrics_StopWorkerRequest(t *testing.T) {
	m := createPrometheusMetrics()
	m.StopWorkerRequest("test_worker", 2*time.Second)

	name := "test_worker"
	if _, ok := m.workerRequestTime[name]; ok {
		t.Fatalf("expected workerRequestTime to not be nil")
	}
}

func TestPrometheusMetrics_StartWorkerRequest(t *testing.T) {
	m := createPrometheusMetrics()
	m.StartWorkerRequest("test_worker")

	name := "test_worker"
	if _, ok := m.busyWorkers[name]; ok {
		t.Fatalf("expected busyWorkers to not be nil")
	}
}
