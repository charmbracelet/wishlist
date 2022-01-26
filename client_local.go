package wishlist

import (
	"context"
	"fmt"
	"os"
	"os/user"
	"path/filepath"

	gossh "golang.org/x/crypto/ssh"
	"golang.org/x/term"
)

type localClient struct{}

func (c *localClient) Connect(e *Endpoint) error {
	resetPty(os.Stdout)
	defer resetPty(os.Stdout)

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get user home dir: %w", err)
	}
	user, err := user.Current()

	if err != nil {
		return fmt.Errorf("failed to get current username: %w", err)
	}

	var methods []gossh.AuthMethod
	if e.IdentityFile == "" {
		methods, err = tryUserKeys(home)
		if err != nil {
			return fmt.Errorf("failed to get user keys: %w", err)
		}
	} else {
		method, err := tryIdentityFile(home, e.IdentityFile)
		if err != nil {
			return fmt.Errorf("failed to parse IdentityFile: %q: %w", e.IdentityFile, err)
		}
		methods = append(methods, method)
	}
	conf := &gossh.ClientConfig{
		User:            firstNonEmpty(e.User, user.Username),
		Auth:            methods,
		HostKeyCallback: hostKeyCallback(e, filepath.Join(home, ".ssh/known_hosts")),
	}

	session, cls, err := createSession(conf, e)
	defer cls.close()
	if err != nil {
		return fmt.Errorf("failed to create sessio: %w", err)
	}

	session.Stdout = os.Stdout
	session.Stderr = os.Stderr
	session.Stdin = os.Stdin

	w, h, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		return fmt.Errorf("failed to get term size: %w", err)
	}

	if err := session.RequestPty(os.Getenv("$TERM"), h, w, nil); err != nil {
		return fmt.Errorf("failed to request a pty: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go c.notifyWindowChanges(ctx, session)

	return shellAndWait(session)
}
