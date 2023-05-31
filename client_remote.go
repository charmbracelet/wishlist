package wishlist

import (
	"fmt"
	"io"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/log"
	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wishlist/blocking"
	gossh "golang.org/x/crypto/ssh"
)

type remoteClient struct {
	// parent session
	session ssh.Session

	// stdin, which is usually multiplexed from the session stdin
	stdin io.Reader

	cleanup func()
}

func (c *remoteClient) For(e *Endpoint) tea.ExecCommand {
	return &remoteSession{
		endpoint:      e,
		parentSession: c.session,
		stdin:         c.stdin,
		cleanup:       c.cleanup,
	}
}

type remoteSession struct {
	// endpoint we are connecting to
	endpoint *Endpoint

	// the parent session (ie the session running the listing)
	parentSession ssh.Session

	stdin   io.Reader
	cleanup func()
}

func (s *remoteSession) SetStdin(_ io.Reader)  {}
func (s *remoteSession) SetStdout(_ io.Writer) {}
func (s *remoteSession) SetStderr(_ io.Writer) {}

func (s *remoteSession) Run() error {
	if s.cleanup != nil {
		s.cleanup()
		defer s.cleanup()
	}
	resetPty(s.parentSession)

	stdin := blocking.New(s.stdin)

	method, agt, closers, err := remoteBestAuthMethod(s.endpoint, s.parentSession, stdin)
	if err != nil {
		return fmt.Errorf("failed to find an auth method: %w", err)
	}
	defer closers.close()

	conf := &gossh.ClientConfig{
		User:            FirstNonEmpty(s.endpoint.User, s.parentSession.User()),
		HostKeyCallback: hostKeyCallback(s.endpoint, ".wishlist/known_hosts"),
		Auth:            method,
		Timeout:         s.endpoint.Timeout,
	}
	session, client, cl, err := createSession(conf, s.endpoint, nil, s.parentSession.Environ()...)
	defer cl.close()
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}

	log.Info(
		"connect",
		"user", s.parentSession.User(),
		"endpoint", s.endpoint.Name,
		"remote.addr", s.parentSession.RemoteAddr().String(),
	)

	session.Stdout = s.parentSession
	session.Stderr = s.parentSession.Stderr()
	session.Stdin = stdin

	if s.endpoint.ForwardAgent {
		if err := forwardAgent(agt, session, client); err != nil {
			return err
		}
	}

	if s.endpoint.RemoteCommand == "" || s.endpoint.RequestTTY {
		log.Info("requesting tty")
		pty, winch, ok := s.parentSession.Pty()
		if !ok {
			return fmt.Errorf("requested a tty, but current session doesn't allow one")
		}
		w := pty.Window
		if err := session.RequestPty(pty.Term, w.Height, w.Width, nil); err != nil {
			return fmt.Errorf("failed to request pty: %w", err)
		}

		done := make(chan bool, 1)
		defer func() { done <- true }()
		go s.notifyWindowChanges(session, done, winch)
	}

	if s.endpoint.RemoteCommand == "" {
		return shellAndWait(session)
	}
	return runAndWait(session, s.endpoint.RemoteCommand)
}

func (s *remoteSession) notifyWindowChanges(session *gossh.Session, done <-chan bool, winch <-chan ssh.Window) {
	for {
		select {
		case <-done:
			return
		case w := <-winch:
			if w.Height == 0 && w.Width == 0 {
				// this only happens if the session is already dead, make sure there are no leftovers
				return
			}
			if err := session.WindowChange(w.Height, w.Width); err != nil {
				log.Warn("failed to notify window change", "err", err)
				return
			}
		}
	}
}
