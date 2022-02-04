//go:build windows
// +build windows

package wishlist

import "golang.org/x/sys/windows"

var (
	Stdin  = int(windows.Stdin)
	Stdout = int(windows.Stdout)
	Stderr = int(windows.Stderr)
)
