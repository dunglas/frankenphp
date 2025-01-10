//go:build darwin
// +build darwin

package memory

func Total() uint64 {
	availBytes, err := sysctlUint64("hw.memsize")
	if err != nil {
		return 0
	}

	return availBytes
}
