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
	fmt.Fprintf(w, termenv.CSI+termenv.EraseDisplaySeq, 2)
}

func mustConnect(s ssh.Session, e *Endpoint, stdin io.Reader) {
	if err := connect(s, e, stdin); err != nil {
		fmt.Fprintf(s, "%s\n\r", err.Error())
		_ = s.Exit(1)
		return // unreachable
	}
	fmt.Fprintf(s, "Closed connection to %q (%s)\n\r", e.Name, e.Address)
	_ = s.Exit(0)
}

func connect(prev ssh.Session, e *Endpoint, stdin io.Reader) error {
	resetPty(prev)

	method, closers, err := authMethod(prev)
	defer closers.close()
	if err != nil {
		return err
	}

	conf := &gossh.ClientConfig{
		User:            firstNonEmpty(e.User, prev.User()),
		HostKeyCallback: hostKeyCallback(e),
		Auth:            []gossh.AuthMethod{method},
	}

	conn, err := gossh.Dial("tcp", e.Address, conf)
	if err != nil {
		return err
	}
	defer func() {
		if err := conn.Close(); err != nil {
			log.Println("failed to close conn:", err)
		}
	}()

	session, err := conn.NewSession()
	if err != nil {
		return err
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
		return err
	}

	done := make(chan bool, 1)
	defer func() { done <- true }()

	go notifyWindowChanges(session, done, winch)

	if err := session.Shell(); err != nil {
		return err
	}

	return session.Wait()
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

	return nil, nil, nil
}

func tryNewKey() (gossh.AuthMethod, error) {
	key, err := keygen.New("", "", nil, keygen.Ed25519)
	if err != nil {
		return nil, err
	}

	signer, err := gossh.ParsePrivateKey(key.PrivateKeyPEM)
	if err != nil {
		return nil, err
	}
	return gossh.PublicKeys(signer), nil
}

func firstNonEmpty(ss ...string) string {
	for _, s := range ss {
		if s != "" {
			return s
		}
	}
	return ""
}

func hostKeyCallback(e *Endpoint) gossh.HostKeyCallback {
	return func(hostname string, remote net.Addr, key gossh.PublicKey) error {
		kh, err := os.OpenFile(".wishlist_known_hosts", os.O_WRONLY|os.O_CREATE, 0600)
		if err != nil {
			return err
		}
		defer kh.Close()

		callback, err := knownhosts.New(kh.Name())
		if err != nil {
			return err
		}

		if err := callback(hostname, remote, key); err != nil {
			var kerr *knownhosts.KeyError
			if errors.As(err, &kerr) {
				if len(kerr.Want) > 0 {
					return fmt.Errorf("possible man-in-the-middle attack: %w", err)
				} else {
					_, err := fmt.Fprintln(kh, knownhosts.Line([]string{e.Address}, key))
					return err
				}
			}
		}
		return nil
	}
}
