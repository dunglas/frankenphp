//go:build !unix

// -build unix

package cpu

import (
	"time"
)

// The fallback always determines that the CPU limits are not reached
func ProbeCPUs(probeTime time.Duration, maxCPUUsage float64, abort chan struct{}) bool {
	timer := time.NewTimer(probeTime)
	select {
	case <-abort:
		return false
	case <-timer.C:
		return true
	}
}
