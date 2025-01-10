package memory

import "golang.org/x/sys/unix"

func TotalSysMemory() uint64 {
	sysInfo := &unix.Sysinfo_t{}
	err := unix.Sysinfo(sysInfo)
	if err != nil {
		return 0
	}

	return uint64(sysInfo.Totalram) * uint64(sysInfo.Unit)
}
