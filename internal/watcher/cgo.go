//go:build !nowatcher

package watcher

// #cgo LDFLAGS: -lwatcher-c -lstdc++
import "C"
