//go:build freebsd || openbsd || dragonfly || netbsd
// +build freebsd openbsd dragonfly netbsd

package memory

// cross platform total system memory
// inspired by: https://github.com/pbnjay/memory
func Total() uint64 {
	availBytes, err := sysctlUint64("hw.physmem")
	if err != nil {
		return 0
	}

	return availBytes
}
