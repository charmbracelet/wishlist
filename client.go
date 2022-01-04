package wishlist

import (
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
	"io"
	"log"

	"github.com/gliderlabs/ssh"
	"github.com/hashicorp/go-multierror"
	"github.com/muesli/termenv"
	gossh "golang.org/x/crypto/ssh"
)

func resetPty(w io.Writer) {
	fmt.Fprint(w, termenv.CSI+termenv.ExitAltScreenSeq)
	fmt.Fprint(w, termenv.CSI+termenv.ResetSeq+"m")
}

func connect(prev ssh.Session, address string) error {
	resetPty(prev)
	defer resetPty(prev)

	_, piv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}

	signer, _ := gossh.ParsePrivateKey(piv)

	conf := &gossh.ClientConfig{
		User:            prev.User(),
		HostKeyCallback: gossh.InsecureIgnoreHostKey(), // TODO: hostkeyCallback,
		Auth: []gossh.AuthMethod{
			gossh.PublicKeys(signer),
		},
	}

	conn, err := gossh.Dial("tcp", address, conf)
	if err != nil {
		return err
	}
	defer conn.Close()

	session, err := conn.NewSession()
	if err != nil {
		return err
	}

	done := make(chan bool, 1)
	var errors error
	defer func() {
		done <- true
		if err := session.Close(); err != nil {
			errors = multierror.Append(errors, err)
		}
	}()

	session.Stdout = prev
	session.Stderr = prev.Stderr()
	session.Stdin = prev

	pty, winch, _ := prev.Pty()
	if err := session.RequestPty(pty.Term, pty.Window.Height, pty.Window.Width, gossh.TerminalModes{}); err != nil {
		return err
	}

	go func() {
		for {
			select {
			case <-done:
				return
			case w := <-winch:
				log.Println("winch on wishlist", w)
				// For some reason, the first resize never gets notified to the channel...
				if err := session.WindowChange(w.Height, w.Width); err != nil {
					log.Println("failed to notify window change", err)
					errors = multierror.Append(errors, err)
					return
				}
			}
		}
	}()

	if err := session.Shell(); err != nil {
		return err
	}

	if err := session.Wait(); err != nil {
		return err
	}

	return errors
}
