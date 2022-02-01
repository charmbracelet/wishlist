package wishlist

import (
	"errors"
	"fmt"
	"io"
	"log"

	gossh "golang.org/x/crypto/ssh"
)

// SSHClient is a SSH client.
type SSHClient interface {
	Connect(e *Endpoint) error
}

func createSession(conf *gossh.ClientConfig, e *Endpoint) (*gossh.Session, *gossh.Client, closers, error) {
	var cl closers
	conn, err := gossh.Dial("tcp", e.Address, conf)
	if err != nil {
		return nil, nil, cl, fmt.Errorf("connection failed: %w", err)
	}

	cl = append(cl, conn.Close)

	session, err := conn.NewSession()
	if err != nil {
		return nil, conn, cl, fmt.Errorf("failed to open session: %w", err)
	}
	cl = append(cl, session.Close)
	return session, conn, cl, nil
}

func shellAndWait(session *gossh.Session) error {
	if err := session.Shell(); err != nil {
		return fmt.Errorf("failed to start shell: %w", err)
	}

	if err := session.Wait(); err != nil {
		return fmt.Errorf("session failed: %w", err)
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
