//go:build windows
// +build windows

package wishlist

import (
	"context"

	"golang.org/x/crypto/ssh"
)

// not available because windows does not implement siscall.SIGWINCH.
func (c *localSession) notifyWindowChanges(ctx context.Context, session *wish.Session) {}
