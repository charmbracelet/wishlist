//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris
// +build darwin dragonfly freebsd linux netbsd openbsd solaris

package wishlist

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/charmbracelet/log"
	"golang.org/x/crypto/ssh"
	"golang.org/x/term"
)

func (s *localSession) notifyWindowChanges(ctx context.Context, session *ssh.Session) {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGWINCH)
	defer func() {
		signal.Stop(sig)
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case <-sig:
			w, h, err := term.GetSize(int(os.Stdout.Fd()))
			if err != nil {
				log.Info("could not get term size", "err", err)
			}
			if err := session.WindowChange(h, w); err != nil {
				log.Info("could not notify term size change", "err", err)
			}
		}
	}
}
