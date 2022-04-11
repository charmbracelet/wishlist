package wishlist

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/user"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/muesli/cancelreader"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/term"
)

// NewLocalSSHClient returns a SSH Client for local usage.
func NewLocalSSHClient() SSHClient {
	return &localClient{}
}

type localClient struct{}

func (c *localClient) Connect(e *Endpoint) (tea.ExecCommand, error) {
	user, err := user.Current()
	if err != nil {
		return nil, fmt.Errorf("failed to get current username: %w", err)
	}

	methods, err := localBestAuthMethod(e)
	if err != nil {
		return nil, fmt.Errorf("failed to setup a authentication method: %w", err)
	}

	session, client, closers, err := createSession(&ssh.ClientConfig{
		User:            firstNonEmpty(e.User, user.Username),
		Auth:            methods,
		HostKeyCallback: hostKeyCallback(e, filepath.Join(user.HomeDir, ".ssh/known_hosts")),
	}, e)
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	closers = append(closers, func() error {
		rc, ok := session.Stdin.(cancelreader.CancelReader)
		if ok && !rc.Cancel() {
			return fmt.Errorf("could not cancel reader")
		}
		return nil
	})
	return &localSession{
		endpoint: e,
		session:  session,
		client:   client,
		closers:  closers,
	}, err
}

type localSession struct {
	// endpoint we are connecting to
	endpoint *Endpoint

	// current session
	session *ssh.Session

	//  client being used in this session
	client *ssh.Client

	// things we need to close
	closers closers
}

func (s *localSession) SetStdin(r io.Reader) {
	rr, err := cancelreader.NewReader(r)
	if err != nil {
		log.Println("failed to create cancel reader", err)
	}
	s.session.Stdin = rr
}

func (s *localSession) SetStdout(w io.Writer) {
	s.session.Stdout = w
}

func (s *localSession) SetStderr(w io.Writer) {
	s.session.Stderr = w
}

func (s *localSession) Run() error {
	defer s.closers.close()

	if s.endpoint.ForwardAgent {
		log.Println("forwarding SSH agent")
		agt, err := getLocalAgent()
		if err != nil {
			return err
		}
		if agt == nil {
			return fmt.Errorf("requested ForwardAgent, but no agent is available")
		}
		if err := agent.RequestAgentForwarding(s.session); err != nil {
			return fmt.Errorf("failed to forward agent: %w", err)
		}
		if err := agent.ForwardToAgent(s.client, agt); err != nil {
			return fmt.Errorf("failed to forward agent: %w", err)
		}
	}

	if s.endpoint.RequestTTY || s.endpoint.RemoteCommand == "" {
		fd := int(os.Stdout.Fd())
		if !term.IsTerminal(fd) {
			return fmt.Errorf("requested a TTY, but current session is not TTY, aborting")
		}

		log.Println("requesting tty")
		originalState, err := term.MakeRaw(fd)
		if err != nil {
			return fmt.Errorf("failed get terminal state: %w", err)
		}

		defer func() {
			if err := term.Restore(fd, originalState); err != nil {
				log.Println("couldn't restore terminal state:", err)
			}
		}()

		w, h, err := term.GetSize(fd)
		if err != nil {
			return fmt.Errorf("failed to get term size: %w", err)
		}

		if err := s.session.RequestPty(os.Getenv("TERM"), h, w, nil); err != nil {
			return fmt.Errorf("failed to request a pty: %w", err)
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go s.notifyWindowChanges(ctx, s.session)
	} else {
		log.Println("did not request a tty")
	}

	if s.endpoint.RemoteCommand == "" {
		return shellAndWait(s.session)
	}
	return runAndWait(s.session, s.endpoint.RemoteCommand)
}
