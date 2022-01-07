package wishlist

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"

	"github.com/charmbracelet/keygen"
	"github.com/gliderlabs/ssh"
	"github.com/muesli/termenv"
	gossh "golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/knownhosts"
)

func resetPty(w io.Writer) {
	fmt.Fprint(w, termenv.CSI+termenv.ExitAltScreenSeq)
	fmt.Fprint(w, termenv.CSI+termenv.ResetSeq+"m")
	fmt.Fprintf(w, termenv.CSI+termenv.EraseDisplaySeq, 2) // nolint:gomnd
}

func mustConnect(s ssh.Session, e *Endpoint, stdin io.Reader) {
	if err := connect(s, e, stdin); err != nil {
		fmt.Fprintf(s, "wishlist: %s\n\r", err.Error())
		_ = s.Exit(1)
		return // unreachable
	}
	fmt.Fprintf(s, "wishlist: closed connection to %q (%s)\n\r", e.Name, e.Address)
	_ = s.Exit(0)
}

func connect(prev ssh.Session, e *Endpoint, stdin io.Reader) error {
	resetPty(prev)

	method, closers, err := authMethod(prev)
	defer closers.close()
	if err != nil {
		return fmt.Errorf("failed to find an auth method: %w", err)
	}

	conf := &gossh.ClientConfig{
		User:            firstNonEmpty(e.User, prev.User()),
		HostKeyCallback: hostKeyCallback(e, ".wishlist/known_hosts"),
		Auth:            []gossh.AuthMethod{method},
	}

	conn, err := gossh.Dial("tcp", e.Address, conf)
	if err != nil {
		return fmt.Errorf("connection failed: %w", err)
	}
	defer func() {
		if err := conn.Close(); err != nil {
			log.Println("failed to close conn:", err)
		}
	}()

	session, err := conn.NewSession()
	if err != nil {
		return fmt.Errorf("failed to open session: %w", err)
	}

	defer func() {
		if err := session.Close(); err != nil && err != io.EOF {
			log.Println("failed to close session:", err)
		}
	}()

	session.Stdout = prev
	session.Stderr = prev.Stderr()
	session.Stdin = stdin

	pty, winch, _ := prev.Pty()
	w := pty.Window
	if err := session.RequestPty(pty.Term, w.Height, w.Width, nil); err != nil {
		return fmt.Errorf("failed to request pty: %w", err)
	}

	done := make(chan bool, 1)
	defer func() { done <- true }()

	go notifyWindowChanges(session, done, winch)

	if err := session.Shell(); err != nil {
		return fmt.Errorf("failed to start shell: %w", err)
	}

	if err := session.Wait(); err != nil {
		return fmt.Errorf("session failed: %w", err)
	}
	return nil
}

func notifyWindowChanges(session *gossh.Session, done <-chan bool, winch <-chan ssh.Window) {
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

type closers []func() error

func (c closers) close() {
	for _, closer := range c {
		if err := closer(); err != nil {
			log.Println("failed to close:", err)
		}
	}
}

// authMethod returns an auth method.
//
// it first tries to use ssh-agent, if that's not available, it creates and uses a new key pair.
func authMethod(s ssh.Session) (gossh.AuthMethod, closers, error) {
	method, closers, err := tryAuthAgent(s)
	if err != nil {
		return method, closers, err
	}
	if method != nil {
		return method, closers, nil
	}

	method, err = tryNewKey()
	return method, closers, err
}

// tryAuthAgent will try to use an ssh-agent to authenticate.
func tryAuthAgent(s ssh.Session) (gossh.AuthMethod, closers, error) {
	_, _ = s.SendRequest("auth-agent-req@openssh.com", true, nil)

	if ssh.AgentRequested(s) {
		l, err := ssh.NewAgentListener()
		if err != nil {
			return nil, nil, err
		}
		go ssh.ForwardAgentConnections(l, s)

		conn, err := net.Dial(l.Addr().Network(), l.Addr().String())
		if err != nil {
			return nil, closers{l.Close}, err
		}

		return gossh.PublicKeysCallback(agent.NewClient(conn).Signers),
			closers{l.Close, conn.Close},
			nil
	}

	fmt.Fprintf(s.Stderr(), "wishlist: ssh agent not available\n\r")
	return nil, nil, nil
}

// tryNewKey will create a .wishlist/client_ed25519 keypair if one does not exist.
// It will return an auth method that uses the keypair if it exist or is successfully created.
func tryNewKey() (gossh.AuthMethod, error) {
	key, err := keygen.New(".wishlist", "client", nil, keygen.Ed25519)
	if err != nil {
		return nil, err
	}

	signer, err := gossh.ParsePrivateKey(key.PrivateKeyPEM)
	if err != nil {
		return nil, err
	}

	if key.IsKeyPairExists() {
		return gossh.PublicKeys(signer), nil
	}

	return gossh.PublicKeys(signer), key.WriteKeys()
}

func firstNonEmpty(ss ...string) string {
	for _, s := range ss {
		if s != "" {
			return s
		}
	}
	return ""
}

// hostKeyCallback returns a callback that will be used to verify the host key.
//
// it creates a file in the given path, and uses that to verify hosts and keys.
// if the host does not exist there, it adds it so its available next time, as plain old `ssh` does.
func hostKeyCallback(e *Endpoint, path string) gossh.HostKeyCallback {
	return func(hostname string, remote net.Addr, key gossh.PublicKey) error {
		kh, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600)
		if err != nil {
			return err
		}
		defer func() { _ = kh.Close() }()

		callback, err := knownhosts.New(kh.Name())
		if err != nil {
			return err
		}

		if err := callback(hostname, remote, key); err != nil {
			var kerr *knownhosts.KeyError
			if errors.As(err, &kerr) {
				if len(kerr.Want) > 0 {
					return fmt.Errorf("possible man-in-the-middle attack: %w", err)
				}
				// if want is empty, it means the host was not in the known_hosts file, so lets add it there.
				_, err := fmt.Fprintln(kh, knownhosts.Line([]string{e.Address}, key))
				return err
			}
		}
		return nil
	}
}
