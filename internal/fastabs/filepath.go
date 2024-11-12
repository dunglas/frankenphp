//go:build !unix

package fastabs

import (
	"path/filepath"
)

// FastAbs can't be optimized on Windows because the
// syscall.FullPath function takes an input.
//
// This function is INTERNAL and must not be used outside of this package.
func FastAbs(path string) (string, error) {
	return filepath.Abs(path)
}
