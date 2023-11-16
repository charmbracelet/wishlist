//go:build windows
// +build windows

package wishlist

import (
	"context"

	"golang.org/x/crypto/ssh"
)

// not available because windows does not implement siscall.SIGWINCH.
func (c *localSession) notifyWindowChanges(ctx context.Context, session *ssh.Session) {}

func makeRaw(fd int) (func(), error) {
	return func() {}, nil
}
