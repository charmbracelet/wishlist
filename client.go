package wishlist

import (
	"fmt"
	"io"
	"log"

	"github.com/muesli/termenv"
	gossh "golang.org/x/crypto/ssh"
)

func createSession(conf *gossh.ClientConfig, e *Endpoint) (*gossh.Session, closers, error) {
	var cl closers
	conn, err := gossh.Dial("tcp", e.Address, conf)
	if err != nil {
		return nil, cl, fmt.Errorf("connection failed: %w", err)
	}

	cl = append(cl, conn.Close)

	session, err := conn.NewSession()
	if err != nil {
		return nil, cl, fmt.Errorf("failed to open session: %w", err)
	}
	cl = append(cl, session.Close)
	return session, cl, nil
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
