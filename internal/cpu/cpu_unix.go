//go:build unix
// +build unix

package cpu

// #include <time.h>
import "C"
import (
	"runtime"
	"time"
)

var cpuCount = runtime.GOMAXPROCS(0)

// probe the CPU usage of the process
// if CPUs are not busy, most threads are likely waiting for I/O, so we should scale
// if CPUs are already busy we won't gain much by scaling and want to avoid the overhead of doing so
func ProbeCPUs(probeTime time.Duration, maxCPUUsage float64, abort chan struct{}) bool {
	var start, end, cpuStart, cpuEnd C.struct_timespec

	// note: clock_gettime is a POSIX function
	// on Windows we'd need to use QueryPerformanceCounter instead
	C.clock_gettime(C.CLOCK_MONOTONIC, &start)
	C.clock_gettime(C.CLOCK_PROCESS_CPUTIME_ID, &cpuStart)

	select {
	case <-abort:
		return false
	case <-time.After(probeTime):
	}

	C.clock_gettime(C.CLOCK_MONOTONIC, &end)
	C.clock_gettime(C.CLOCK_PROCESS_CPUTIME_ID, &cpuEnd)

	elapsedTime := float64(end.tv_sec-start.tv_sec)*1e9 + float64(end.tv_nsec-start.tv_nsec)
	elapsedCpuTime := float64(cpuEnd.tv_sec-cpuStart.tv_sec)*1e9 + float64(cpuEnd.tv_nsec-cpuStart.tv_nsec)
	cpuUsage := elapsedCpuTime / elapsedTime / float64(cpuCount)

	return cpuUsage < maxCPUUsage
}
