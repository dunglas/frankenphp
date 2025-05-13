//go:build include_php_cli

package main

import (
	_ "embed"
	"fmt"
	"golang.org/x/sys/unix"
	"os"
	"syscall"
)

//go:embed php-cli
var phpcli []byte

func runPhpCli() {
	// use some linux voodoo to exec into the embedded php-cli
	// we do a syscall: SYS_MEMFD_CREATE
	// this will create an anonymous in-ram file
	name := "phpcli"
	fd, err := unix.MemfdCreate(name, unix.MFD_CLOEXEC|unix.MFD_ALLOW_SEALING)
	if err != nil {
		fmt.Fprintf(os.Stderr, "memfd_create failed: %v\n", err)
		os.Exit(1)
	}
	defer func(fd int) {
		// theoretically never called on the happy path.
		err := unix.Close(fd)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to close php-cli process: %v\n", err)
			os.Exit(1)
		}
	}(fd)

	// now we copy our embedded php-cli to the anonymous file
	_, err = unix.Write(fd, phpcli)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to initialize php-cli: %v\n", err)
		os.Exit(1)
	}

	// and make it executable
	err = unix.Fchmod(fd, 0755)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to initialize php-cli: %v\n", err)
		os.Exit(1)
	}

	// now seal it so it cannot be written to or any other seals
	_, _, errno := syscall.Syscall(syscall.SYS_FCNTL, uintptr(fd), unix.F_ADD_SEALS, unix.F_SEAL_WRITE|unix.F_SEAL_SEAL)
	if errno != 0 {
		fmt.Fprintf(os.Stderr, "failed to seal php-cli memory from tampering: %v\n", errno)
		os.Exit(1)
	}

	path := fmt.Sprintf("/proc/self/fd/%d", fd)
	err = syscall.Exec(path, []string{path}, os.Environ())
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to exec into php-cli: %v\n", err)
		os.Exit(1)
	}

	// this is never reached
}
