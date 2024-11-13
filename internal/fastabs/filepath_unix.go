//go:build unix

package fastabs

import (
	"os"
	"path/filepath"
)

// FastAbs is an optimized version of filepath.Abs for Unix systems,
// since we don't expect the working directory to ever change once
// Caddy is running. Avoid the os.Getwd syscall overhead.
func FastAbs(path string) (string, error) {
	if filepath.IsAbs(path) {
		return filepath.Clean(path), nil
	}

	if wderr != nil {
		return "", wderr
	}

	return filepath.Join(wd, path), nil
}

var wd, wderr = os.Getwd()
