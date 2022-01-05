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

	var errors error

	conn, err := gossh.Dial("tcp", address, conf)
	if err != nil {
		return err
	}
	defer func() {
		log.Println("closing conn")
		if err := conn.Close(); err != nil {
			errors = multierror.Append(errors, err)
		}
	}()

	session, err := conn.NewSession()
	if err != nil {
		return err
	}

	defer func() {
		log.Println("closing session")
		if err := session.Close(); err != nil {
			errors = multierror.Append(errors, err)
		}
	}()

	session.Stdout = prev
	session.Stderr = prev.Stderr()
	session.Stdin = prev

	pty, winch, _ := prev.Pty()
	w := pty.Window
	if err := session.RequestPty(pty.Term, w.Height, w.Width, nil); err != nil {
		return err
	}

	done := make(chan bool, 1)
	defer func() { done <- true }()

	go notifyWindowChanges(session, done, winch)

	// Non blocking:
	// - session.Shell()
	// - session.Start()
	//
	// Blocking:
	// - session.Run()
	// - session.Output()
	// - session.CombinedOutput()
	// - session.Wait()
	//
	if err := session.Run(""); err != nil {
		return err
	}

	return errors
}

func notifyWindowChanges(session *gossh.Session, done <-chan bool, winch <-chan ssh.Window) {
	for {
		select {
		case <-done:
			log.Println("winch done")
			return
		case w := <-winch:
			if w.Height == 0 && w.Width == 0 {
				// this only happens if the session is already dead, make sure there are no leftovers
				return
			}
			log.Println("resize", w)
			if err := session.WindowChange(w.Height, w.Width); err != nil {
				log.Println("failed to notify window change", err)
				return
			}
		}
	}
}
