package wishlist

import (
	"fmt"
	"io"
	"log"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/wishlist/blocking"
	"github.com/gliderlabs/ssh"
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

func (s *remoteSession) SetStdin(r io.Reader)  {}
func (s *remoteSession) SetStdout(w io.Writer) {}
func (s *remoteSession) SetStderr(w io.Writer) {}

func (s *remoteSession) Run() error {
	if s.cleanup != nil {
		s.cleanup()
		defer s.cleanup()
	}
	resetPty(s.parentSession)

	method, agt, closers, err := remoteBestAuthMethod(s.parentSession)
	if err != nil {
		return fmt.Errorf("failed to find an auth method: %w", err)
	}
	defer closers.close()

	conf := &gossh.ClientConfig{
		User:            firstNonEmpty(s.endpoint.User, s.parentSession.User()),
		HostKeyCallback: hostKeyCallback(s.endpoint, ".wishlist/known_hosts"),
		Auth:            []gossh.AuthMethod{method},
		Timeout:         s.endpoint.Timeout,
	}
	session, client, cl, err := createSession(conf, s.endpoint, nil, s.parentSession.Environ()...)
	defer cl.close()
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}

	log.Printf("%s connect to %q, %s", s.parentSession.User(), s.endpoint.Name, s.parentSession.RemoteAddr().String())

	session.Stdout = s.parentSession
	session.Stderr = s.parentSession.Stderr()
	session.Stdin = blocking.New(s.stdin)

	if s.endpoint.ForwardAgent {
		if err := forwardAgent(agt, session, client); err != nil {
			return err
		}
	}

	if s.endpoint.RemoteCommand == "" || s.endpoint.RequestTTY {
		log.Println("requesting tty")
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
				log.Println("failed to notify window change:", err)
				return
			}
		}
	}
}
