//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris
// +build darwin dragonfly freebsd linux netbsd openbsd solaris

package wishlist

import "golang.org/x/sys/unix"

var (
	Stdin  = unix.Stdin
	Stdout = unix.Stdout
	Stderr = unix.Stderr
)
