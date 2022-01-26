package wishlist

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"os/user"
	"path/filepath"
	"syscall"

	"github.com/gliderlabs/ssh"
	"github.com/muesli/termenv"
	gossh "golang.org/x/crypto/ssh"
	"golang.org/x/term"
)

type remoteClient struct {
	session ssh.Session
	stdin   io.Reader
}

func (c *remoteClient) Connect(e *Endpoint) error {
	resetPty(c.session)

	method, closers, err := remoteBestAuthMethod(c.session)
	defer closers.close()
	if err != nil {
		return fmt.Errorf("failed to find an auth method: %w", err)
	}

	conf := &gossh.ClientConfig{
		User:            firstNonEmpty(e.User, c.session.User()),
		HostKeyCallback: hostKeyCallback(e, ".wishlist/known_hosts"),
		Auth:            []gossh.AuthMethod{method},
	}

	session, cl, err := createSession(conf, e)
	defer cl.close()
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}

	log.Printf("%s connect to %q, %s", c.session.User(), e.Name, c.session.RemoteAddr().String())

	session.Stdout = c.session
	session.Stderr = c.session.Stderr()
	session.Stdin = c.stdin

	pty, winch, _ := c.session.Pty()
	w := pty.Window
	if err := session.RequestPty(pty.Term, w.Height, w.Width, nil); err != nil {
		return fmt.Errorf("failed to request pty: %w", err)
	}

	done := make(chan bool, 1)
	defer func() { done <- true }()

	go c.notifyWindowChanges(session, done, winch)

	return shellAndWait(session)
}

func (c *remoteClient) notifyWindowChanges(session *gossh.Session, done <-chan bool, winch <-chan ssh.Window) {
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

	methods, err := tryUserKeys(home)
	if err != nil {
		return fmt.Errorf("failed to get user keys: %w", err)
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

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGWINCH)
	defer func() {
		signal.Stop(sig)
	}()

	go func() {
		for range sig {
			w, h, err := term.GetSize(int(os.Stdout.Fd()))
			if err != nil {
				log.Println(err)
			}
			if err := session.WindowChange(h, w); err != nil {
				log.Println(err)
			}
		}
	}()

	return shellAndWait(session)
}

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
