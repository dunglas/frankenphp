//go:build !unix

package fastabs

import (
	"path/filepath"
)

// FastAbs can't be optimized on Windows because the
// syscall.FullPath function takes an input.
func FastAbs(path string) (string, error) {
	return filepath.Abs(path)
}
