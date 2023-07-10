package wishlist

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/log"
	"github.com/muesli/termenv"
	gossh "golang.org/x/crypto/ssh"
)

// SSHClient is a SSH client.
type SSHClient interface {
	For(e *Endpoint) tea.ExecCommand
}

func proxyJump(addr, nextAddr string, conf, nextConf *gossh.ClientConfig) (*gossh.Client, closers, error) {
	var cl closers
	log.Info("connecting client to ProxyJump", "addr", addr)
	jumpClient, err := gossh.Dial("tcp", addr, conf)
	if err != nil {
		return nil, cl, fmt.Errorf("connection to ProxyJump (%q) failed: %w", addr, err)
	}
	cl = append(cl, jumpClient.Close)

	log.Info("connecting to target using jump client", "addr", nextAddr)
	jumpConn, err := jumpClient.Dial("tcp", nextAddr)
	if err != nil {
		return nil, cl, fmt.Errorf("connection from ProxyJump (%q) to Host (%q) failed: %w", addr, nextAddr, err)
	}
	cl = append(cl, jumpConn.Close)

	log.Info("getting client connection", "addr", nextAddr)
	ncc, chans, reqs, err := gossh.NewClientConn(jumpConn, nextAddr, nextConf)
	if err != nil {
		return nil, cl, fmt.Errorf("client connection from ProxyJump (%q) to Host (%q) failed: %w", addr, nextAddr, err)
	}
	cl = append(cl, ncc.Close)

	return gossh.NewClient(ncc, chans, reqs), cl, nil
}

func createSession(conf *gossh.ClientConfig, e *Endpoint, abort <-chan os.Signal, env ...string) (*gossh.Session, *gossh.Client, closers, error) {
	var cl closers
	var conn *gossh.Client
	var err error
	connected := make(chan bool, 1)

	if jump := e.ProxyJump; jump == "" {
		go func() {
			conn, err = gossh.Dial("tcp", e.Address, conf)
			connected <- true
		}()
	} else {
		jumpConf := &gossh.ClientConfig{
			User:            conf.User,
			Auth:            conf.Auth,
			HostKeyCallback: conf.HostKeyCallback,
		}
		username, addr, ok := strings.Cut(jump, "@")
		if ok {
			jumpConf.User = username
		} else {
			addr = jump
		}
		log.Info("ProxyJump", "jump", jump, "addr", addr, "user", jumpConf.User)

		go func() {
			conn, cl, err = proxyJump(addr, e.Address, jumpConf, conf)
			connected <- true
		}()
	}

	select {
	case <-connected:
		// fallback
		break
	case <-abort:
		if conn != nil {
			cl = append(cl, conn.Close)
		}
		return nil, nil, cl, fmt.Errorf("connection aborted")
	}

	if err != nil {
		return nil, nil, cl, fmt.Errorf("connection failed: %w", err)
	}
	cl = append(cl, conn.Close)

	session, err := conn.NewSession()
	if err != nil {
		return nil, conn, cl, fmt.Errorf("failed to open session: %w", err)
	}
	cl = append(cl, session.Close)
	for k, v := range e.Environment(env...) {
		if err := session.Setenv(k, v); err != nil {
			log.Warn("could not set env", "key", k, "value", v, "err", err)
			continue
		}
		log.Info("setting env", "key", k, "value", v)
	}
	return session, conn, cl, nil
}

func shellAndWait(session *gossh.Session) error {
	log.Info("requesting shell")
	if err := session.Shell(); err != nil {
		return fmt.Errorf("failed to start shell: %w", err)
	}
	if err := session.Wait(); err != nil {
		if errors.Is(err, &gossh.ExitMissingError{}) {
			log.Info("exit was missing, assuming exit 0")
			return nil
		}
		return fmt.Errorf("session failed: %w", err)
	}
	return nil
}

func runAndWait(session *gossh.Session, cmd string) error {
	log.Info("running", "command", cmd)
	if err := session.Run(cmd); err != nil {
		return fmt.Errorf("failed to run %q: %w", cmd, err)
	}
	return nil
}

type closers []func() error

func (c closers) close() {
	for _, closer := range c {
		if err := closer(); err != nil {
			if errors.Is(err, io.EOF) {
				// do not print EOF errors... not a big deal anyway
				continue
			}
			log.Warn("failed to close", "err", err)
		}
	}
}

func resetPty(w io.Writer) {
	fmt.Fprint(w, termenv.CSI+termenv.ExitAltScreenSeq)
	fmt.Fprint(w, termenv.CSI+termenv.ResetSeq+"m")
	fmt.Fprintf(w, termenv.CSI+termenv.EraseDisplaySeq, 2) //nolint:gomnd
}
