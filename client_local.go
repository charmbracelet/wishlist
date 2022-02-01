package wishlist

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/user"
	"path/filepath"

	gossh "golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/term"
)

// NewLocalSSHClient returns a SSH Client for local usage.
func NewLocalSSHClient() SSHClient {
	return &localClient{}
}

type localClient struct{}

func (c *localClient) Connect(e *Endpoint) error {
	user, err := user.Current()
	if err != nil {
		return fmt.Errorf("failed to get current username: %w", err)
	}

	methods, err := localBestAuthMethod(e)
	if err != nil {
		return fmt.Errorf("failed to setup a authentication method: %w", err)
	}
	conf := &gossh.ClientConfig{
		User:            firstNonEmpty(e.User, user.Username),
		Auth:            methods,
		HostKeyCallback: hostKeyCallback(e, filepath.Join(user.HomeDir, ".ssh/known_hosts")),
	}

	session, client, cls, err := createSession(conf, e)
	defer cls.close()
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}

	session.Stdout = os.Stdout
	session.Stderr = os.Stderr
	session.Stdin = os.Stdin

	if e.ForwardAgent {
		log.Println("forwarding SSH agent")
		agt, err := getLocalAgent()
		if err != nil {
			return err
		}
		if agt == nil {
			return fmt.Errorf("requested ForwardAgent, but no agent is available")
		}
		if err := agent.RequestAgentForwarding(session); err != nil {
			return fmt.Errorf("failed to forward agent: %w", err)
		}
		if err := agent.ForwardToAgent(client, agt); err != nil {
			return fmt.Errorf("failed to forward agent: %w", err)
		}
	}

	if e.RequestTTY || e.RemoteCommand == "" {
		log.Println("requesting tty")
		w, h, err := term.GetSize(int(os.Stdout.Fd()))
		if err != nil {
			return fmt.Errorf("failed to get term size: %w", err)
		}

		if err := session.RequestPty(os.Getenv("TERM"), h, w, nil); err != nil {
			return fmt.Errorf("failed to request a pty: %w", err)
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go c.notifyWindowChanges(ctx, session)
	} else {
		log.Println("did not request a tty")
	}

	if e.RemoteCommand == "" {
		return shellAndWait(session)
	}
	return runAndWait(session, e.RemoteCommand)
}
