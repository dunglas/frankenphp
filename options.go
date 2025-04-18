package frankenphp

import (
	"time"

	"go.uber.org/zap"
)

// Option instances allow to configure FrankenPHP.
type Option func(h *opt) error

// opt contains the available options.
//
// If you change this, also update the Caddy module and the documentation.
type opt struct {
	numThreads  int
	maxThreads  int
	workers     []workerOpt
	logger      *zap.Logger
	metrics     Metrics
	phpIni      map[string]string
	maxWaitTime time.Duration
}

type workerOpt struct {
	name     string
	fileName string
	num      int
	env      PreparedEnv
	watch    []string
	moduleID uintptr
}

// WithNumThreads configures the number of PHP threads to start.
func WithNumThreads(numThreads int) Option {
	return func(o *opt) error {
		o.numThreads = numThreads

		return nil
	}
}

func WithMaxThreads(maxThreads int) Option {
	return func(o *opt) error {
		o.maxThreads = maxThreads

		return nil
	}
}

func WithMetrics(m Metrics) Option {
	return func(o *opt) error {
		o.metrics = m

		return nil
	}
}

// WithWorkers configures the PHP workers to start, moduleID is used to identify the worker for a specific domain
func WithWorkers(name string, fileName string, num int, env map[string]string, watch []string, moduleID uintptr) Option {
	return func(o *opt) error {
		o.workers = append(o.workers, workerOpt{name, fileName, num, PrepareEnv(env), watch, moduleID})

		return nil
	}
}

// WithLogger configures the global logger to use.
func WithLogger(l *zap.Logger) Option {
	return func(o *opt) error {
		o.logger = l

		return nil
	}
}

// WithPhpIni configures user defined PHP ini settings.
func WithPhpIni(overrides map[string]string) Option {
	return func(o *opt) error {
		o.phpIni = overrides
		return nil
	}
}

// WithMaxWaitTime configures the max time a request may be stalled waiting for a thread.
func WithMaxWaitTime(maxWaitTime time.Duration) Option {
	return func(o *opt) error {
		o.maxWaitTime = maxWaitTime

		return nil
	}
}
