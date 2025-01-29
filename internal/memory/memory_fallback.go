//go:build !linux

package memory

// Return 0 if the total system memory cannot be determined
func TotalSysMemory() uint64 {
	return 0
}
