//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris
// +build darwin dragonfly freebsd linux netbsd openbsd solaris

package wishlist

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"golang.org/x/crypto/ssh"
	"golang.org/x/term"
)

func (c *localClient) notifyWindowChanges(ctx context.Context, session *ssh.Session) {
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
				log.Println(err)
			}
			if err := session.WindowChange(h, w); err != nil {
				log.Println(err)
			}
		}
	}
}
