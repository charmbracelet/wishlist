package wishlist

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"os/user"
	"path/filepath"
	"runtime"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/log"
	"github.com/muesli/cancelreader"
	"golang.org/x/crypto/ssh"
	"golang.org/x/term"
)

// NewLocalSSHClient returns a SSH Client for local usage.
func NewLocalSSHClient() SSHClient {
	return &localClient{}
}

type localClient struct{}

func (c *localClient) For(e *Endpoint) tea.ExecCommand {
	return &localSession{
		endpoint: e,
	}
}

type localSession struct {
	// endpoint we are connecting to
	endpoint *Endpoint

	stdin          io.Reader
	stdout, stderr io.Writer
}

func (s *localSession) SetStdin(r io.Reader) {
	s.stdin = r
}

func (s *localSession) SetStdout(w io.Writer) {
	s.stdout = w
}

func (s *localSession) SetStderr(w io.Writer) {
	s.stderr = w
}

func (s *localSession) Run() error {
	resetPty(s.stdout)

	abort := make(chan os.Signal, 1)
	signal.Notify(abort, os.Interrupt)
	defer func() {
		signal.Stop(abort)
		close(abort)
	}()

	user, err := user.Current()
	if err != nil {
		return fmt.Errorf("failed to get current username: %w", err)
	}

	agt, cls, err := getLocalAgent()
	if err != nil {
		return err
	}
	defer cls.close()

	methods, err := localBestAuthMethod(agt, s.endpoint, os.Stdin, os.Stdout)
	if err != nil {
		return fmt.Errorf("failed to setup a authentication method: %w", err)
	}

	conf := &ssh.ClientConfig{
		User:            FirstNonEmpty(s.endpoint.User, user.Username),
		Auth:            methods,
		HostKeyCallback: hostKeyCallback(s.endpoint, filepath.Join(user.HomeDir, ".ssh/known_hosts")),
		Timeout:         s.endpoint.Timeout,
	}

	session, client, cls, err := createSession(conf, s.endpoint, abort, os.Environ()...)
	defer cls.close()
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	defer closers{func() error {
		rc, ok := session.Stdin.(cancelreader.CancelReader)
		if ok && !rc.Cancel() {
			return fmt.Errorf("could not cancel reader")
		}
		return nil
	}}.close()

	session.Stdout = s.stdout
	session.Stderr = s.stderr
	stdin, err := cancelreader.NewReader(s.stdin)
	if err != nil {
		return fmt.Errorf("could not create cancel reader")
	}
	session.Stdin = stdin

	if s.endpoint.ForwardAgent {
		if err := forwardAgent(agt, session, client); err != nil {
			return err
		}
	}

	//nolint:nestif
	if s.endpoint.RequestTTY || s.endpoint.RemoteCommand == "" {
		fd := int(os.Stdout.Fd())
		if !term.IsTerminal(fd) {
			return fmt.Errorf("requested a TTY, but current session is not TTY, aborting")
		}

		log.Info("requesting tty")
		if runtime.GOOS != "windows" {
			originalState, err := term.MakeRaw(fd)
			if err != nil {
				return fmt.Errorf("failed get terminal state: %w", err)
			}

			defer func() {
				if err := term.Restore(fd, originalState); err != nil {
					log.Warn("couldn't restore terminal state", "err", err)
				}
			}()
		}

		w, h, err := term.GetSize(fd)
		if err != nil {
			return fmt.Errorf("failed to get term size: %w", err)
		}

		if err := session.RequestPty(os.Getenv("TERM"), h, w, nil); err != nil {
			return fmt.Errorf("failed to request a pty: %w", err)
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go s.notifyWindowChanges(ctx, session)
	} else {
		log.Info("did not request a tty")
	}

	if s.endpoint.RemoteCommand == "" {
		return shellAndWait(session)
	}
	return runAndWait(session, s.endpoint.RemoteCommand)
}
