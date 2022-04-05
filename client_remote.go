package wishlist

import (
	"fmt"
	"io"
	"log"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gliderlabs/ssh"
	"github.com/muesli/termenv"
	gossh "golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

type remoteClient struct {
	session ssh.Session
}

type remoteSession struct {
	endpoint      *Endpoint
	parentSession ssh.Session
	session       *gossh.Session
	client        *gossh.Client
	closers       closers
	agent         agent.Agent
}

func (s *remoteSession) SetStdin(r io.Reader) {
	s.session.Stdin = r
}

func (s *remoteSession) SetStdout(w io.Writer) {
	s.session.Stdout = w
}

func (s *remoteSession) SetStderr(w io.Writer) {
	s.session.Stderr = w
}

func (s *remoteSession) Run() error {
	if s.endpoint.ForwardAgent {
		log.Println("forwarding SSH agent")
		if s.agent == nil {
			return fmt.Errorf("requested ForwardAgent, but no agent is available")
		}
		if err := agent.RequestAgentForwarding(s.session); err != nil {
			return fmt.Errorf("failed to forward agent: %w", err)
		}
		if err := agent.ForwardToAgent(s.client, s.agent); err != nil {
			return fmt.Errorf("failed to forward agent: %w", err)
		}
	}

	if s.endpoint.RemoteCommand == "" || s.endpoint.RequestTTY {
		log.Println("requesting tty")
		pty, winch, ok := s.parentSession.Pty()
		if !ok {
			return fmt.Errorf("requested a tty, but current session doesn't allow one")
		}
		w := pty.Window
		if err := s.session.RequestPty(pty.Term, w.Height, w.Width, nil); err != nil {
			return fmt.Errorf("failed to request pty: %w", err)
		}

		done := make(chan bool, 1)
		defer func() { done <- true }()
		go s.notifyWindowChanges(s.session, done, winch)
	}

	if s.endpoint.RemoteCommand == "" {
		return shellAndWait(s.session)
	}
	return runAndWait(s.session, s.endpoint.RemoteCommand)
}

func (c *remoteClient) Connect(e *Endpoint) (tea.ExecCommand, error) {
	resetPty(c.session)

	method, agt, closers, err := remoteBestAuthMethod(c.session)
	if err != nil {
		return nil, fmt.Errorf("failed to find an auth method: %w", err)
	}

	session, client, cl, err := createSession(&gossh.ClientConfig{
		User:            firstNonEmpty(e.User, c.session.User()),
		HostKeyCallback: hostKeyCallback(e, ".wishlist/known_hosts"),
		Auth:            []gossh.AuthMethod{method},
	}, e)
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	closers = append(closers, cl...)

	log.Printf("%s connect to %q, %s", c.session.User(), e.Name, c.session.RemoteAddr().String())
	return &remoteSession{
		endpoint:      e,
		parentSession: c.session,
		session:       session,
		client:        client,
		closers:       closers,
		agent:         agt,
	}, nil
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

func resetPty(w io.Writer) {
	fmt.Fprint(w, termenv.CSI+termenv.ExitAltScreenSeq)
	fmt.Fprint(w, termenv.CSI+termenv.ResetSeq+"m")
	fmt.Fprintf(w, termenv.CSI+termenv.EraseDisplaySeq, 2) // nolint:gomnd
}
