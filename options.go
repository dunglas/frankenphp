package frankenphp

import (
	"go.uber.org/zap"
)

// Option instances allow to configure the SAP.
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
}

// WithNumThreads allows to configure the number of PHP threads to start (worker threads excluded).
func WithNumThreads(numThreads int) Option {
	return func(o *opt) error {
		o.numThreads = numThreads

		return nil
	}
}

// WithWorkers allow to start worker goroutines.
func WithWorkers(fileName string, num int) Option {
	return func(o *opt) error {
		o.workers = append(o.workers, workerOpt{fileName, num})

		return nil
	}
}

// WithLogger sets the global logger to use
func WithLogger(l *zap.Logger) Option {
	return func(o *opt) error {
		o.logger = l

		return nil
	}
}
