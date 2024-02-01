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
}

type workerOpt struct {
	fileName string
	num      int
	env      PreparedEnv
}

// WithNumThreads configures the number of PHP threads to start.
func WithNumThreads(numThreads int) Option {
	return func(o *opt) error {
		o.numThreads = numThreads

		return nil
	}
}

// WithWorkers configures the PHP workers to start.
func WithWorkers(fileName string, num int, env map[string]string) Option {
	return func(o *opt) error {
		o.workers = append(o.workers, workerOpt{fileName, num, PrepareEnv(env)})

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
