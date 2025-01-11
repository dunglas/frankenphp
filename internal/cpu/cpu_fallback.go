//go:build !unix

// -build unix

package cpu

import (
	"time"
)

// The fallback always determines that the CPU limits are not reached
func ProbeCPUs(probeTime time.Duration, maxCPUUsage float64, abort chan struct{}) bool {
	select {
	case <-abort:
		return false
	case <-time.After(probeTime):
		return true
	}
}
