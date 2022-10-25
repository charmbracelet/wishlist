package wishlist

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/muesli/termenv"
	"golang.org/x/crypto/ssh"
	gossh "golang.org/x/crypto/ssh"
)

// SSHClient is a SSH client.
type SSHClient interface {
	For(e *Endpoint) tea.ExecCommand
}

func createSession(conf *gossh.ClientConfig, e *Endpoint, abort <-chan os.Signal, env ...string) (*gossh.Session, *gossh.Client, closers, error) {
	var cl closers
	var conn *gossh.Client
	var err error
	connected := make(chan bool, 1)

	go func() {
		conn, err = gossh.Dial("tcp", e.Address, conf)
		connected <- true
	}()

	select {
	case <-connected:
		// fallback
		break
	case <-abort:
		if conn != nil {
			_ = conn.Close()
		}
		return nil, nil, nil, fmt.Errorf("connection aborted")
	}

	if err != nil {
		return nil, nil, cl, fmt.Errorf("connection failed: %w", err)
	}
	cl = append(cl, conn.Close)

	session, err := conn.NewSession()
	if err != nil {
		return nil, conn, cl, fmt.Errorf("failed to open session: %w", err)
	}
	cl = append(cl, session.Close)
	for k, v := range e.Environment(env...) {
		if err := session.Setenv(k, v); err != nil {
			return session, conn, cl, fmt.Errorf("could not set env: %q: %w", env, err)
		}
		log.Printf("setting env %s = %q", k, v)
	}
	return session, conn, cl, nil
}

func shellAndWait(session *gossh.Session) error {
	log.Println("requesting shell")
	if err := session.Shell(); err != nil {
		return fmt.Errorf("failed to start shell: %w", err)
	}
	if err := session.Wait(); err != nil && !errors.Is(err, &ssh.ExitMissingError{}) {
		return fmt.Errorf("session failed: %w", err)
	}
	return nil
}

func runAndWait(session *gossh.Session, cmd string) error {
	log.Printf("running %q", cmd)
	if err := session.Run(cmd); err != nil {
		return fmt.Errorf("failed to run %q: %w", cmd, err)
	}
	return nil
}

type closers []func() error

func (c closers) close() {
	for _, closer := range c {
		if err := closer(); err != nil {
			if errors.Is(err, io.EOF) {
				// do not print EOF errors... not a big deal anyway
				continue
			}
			log.Println("failed to close:", err)
		}
	}
}

func firstNonEmpty(ss ...string) string {
	for _, s := range ss {
		if s != "" {
			return s
		}
	}
	return ""
}

func resetPty(w io.Writer) {
	fmt.Fprint(w, termenv.CSI+termenv.ExitAltScreenSeq)
	fmt.Fprint(w, termenv.CSI+termenv.ResetSeq+"m")
	fmt.Fprintf(w, termenv.CSI+termenv.EraseDisplaySeq, 2) // nolint:gomnd
}
