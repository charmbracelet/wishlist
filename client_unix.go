//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris
// +build darwin dragonfly freebsd linux netbsd openbsd solaris

package wishlist

import (
	"context"
	"fmt"
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
			// #nosec G115
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

func makeRaw(fd int) (func(), error) {
	log.Info("putting term in raw mode")
	originalState, err := term.MakeRaw(fd)
	if err != nil {
		return func() {}, fmt.Errorf("failed get terminal state: %w", err)
	}

	return func() {
		if err := term.Restore(fd, originalState); err != nil {
			log.Warn("couldn't restore terminal state", "err", err)
		}
	}, nil
}
