package wishlist

import (
	"fmt"
	"io"
	"log"

	"github.com/charmbracelet/keygen"
	"github.com/gliderlabs/ssh"
	"github.com/muesli/termenv"
	gossh "golang.org/x/crypto/ssh"
)

func resetPty(w io.Writer) {
	fmt.Fprint(w, termenv.CSI+termenv.ExitAltScreenSeq)
	fmt.Fprint(w, termenv.CSI+termenv.ResetSeq+"m")
}

func mustConnect(s ssh.Session, e *Endpoint) {
	if err := connect(s, e); err != nil {
		fmt.Fprintln(s, err.Error())
		s.Exit(1)
		return //unreachable
	}
	fmt.Fprintf(s, "Closed connection to %q (%s)\n", e.Name, e.Address)
	s.Exit(0)
}

func connect(prev ssh.Session, e *Endpoint) error {
	resetPty(prev)
	defer resetPty(prev)

	key, err := keygen.New("", "", nil, keygen.Ed25519)
	if err != nil {
		return err
	}

	signer, err := gossh.ParsePrivateKey(key.PrivateKeyPEM)
	if err != nil {
		return err
	}

	conf := &gossh.ClientConfig{
		User:            prev.User(),
		HostKeyCallback: gossh.InsecureIgnoreHostKey(),
		Auth:            []gossh.AuthMethod{gossh.PublicKeys(signer)},
	}

	conn, err := gossh.Dial("tcp", e.Address, conf)
	if err != nil {
		return err
	}
	defer func() {
		if err := conn.Close(); err != nil {
			log.Println("failed to close conn:", err)
		}
	}()

	session, err := conn.NewSession()
	if err != nil {
		return err
	}

	defer func() {
		if err := session.Close(); err != nil {
			log.Println("failed to close session:", err)
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
	if err := session.Shell(); err != nil {
		return err
	}

	return session.Wait()
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
