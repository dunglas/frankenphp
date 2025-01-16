//go:build linux

package memory

import "syscall"

func TotalSysMemory() uint64 {
	sysInfo := &syscall.Sysinfo_t{}
	err := syscall.Sysinfo(sysInfo)
	if err != nil {
		return 0
	}

	return uint64(sysInfo.Totalram) * uint64(sysInfo.Unit)
}
