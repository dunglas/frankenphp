package frankenphp

import (
	"go.uber.org/zap"
)

// Option instances allow to configure FrankenPHP.
type Option func(h *opt) error

// opt contains the available options.
//
// If you change this, also update the Caddy module and the documentation.
type opt struct {
	numThreads int
	workers    []workerOpt
	logger     *zap.Logger
	metrics    Metrics
	// sapi options
	phpIniIgnore    bool
	phpIniIgnoreCwd bool
}

type workerOpt struct {
	fileName string
	num      int
	env      PreparedEnv
	watch    []string
}

// WithNumThreads configures the number of PHP threads to start.
func WithNumThreads(numThreads int) Option {
	return func(o *opt) error {
		o.numThreads = numThreads

		return nil
	}
}

func WithMetrics(m Metrics) Option {
	return func(o *opt) error {
		o.metrics = m

		return nil
	}
}

// WithWorkers configures the PHP workers to start.
func WithWorkers(fileName string, num int, env map[string]string, watch []string) Option {
	return func(o *opt) error {
		o.workers = append(o.workers, workerOpt{fileName, num, PrepareEnv(env), watch})

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

// WithPHPIniIgnore don't look for php.ini
func WithPHPIniIgnore(ignore bool) Option {
	return func(o *opt) error {
		o.phpIniIgnore = ignore

		return nil
	}
}

// WithPHPIniIgnoreCwd don't look for php.ini in the current directory
func WithPHPIniIgnoreCwd(ignoreCwd bool) Option {
	return func(o *opt) error {
		o.phpIniIgnoreCwd = ignoreCwd

		return nil
	}
}
