package wishlist

import (
	"crypto/ed25519"
	"crypto/rand"
	"log"

	"github.com/gliderlabs/ssh"
	gossh "golang.org/x/crypto/ssh"
)

func connect(prev ssh.Session, address string) error {
	_, piv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}

	signer, _ := gossh.ParsePrivateKey(piv)

	conf := &gossh.ClientConfig{
		User:            prev.User(),
		HostKeyCallback: gossh.InsecureIgnoreHostKey(), // TODO: hostkeyCallback,
		Auth: []gossh.AuthMethod{
			gossh.PublicKeys(signer),
		},
	}

	conn, err := gossh.Dial("tcp", address, conf)
	if err != nil {
		return err
	}
	defer conn.Close()

	session, err := conn.NewSession()
	if err != nil {
		return err
	}
	defer session.Close()

	session.Stdout = prev
	session.Stderr = prev.Stderr()
	session.Stdin = prev

	pty, _, _ := prev.Pty()
	log.Println("requesting pty", pty)
	if err := session.RequestPty(pty.Term, pty.Window.Width, pty.Window.Height, gossh.TerminalModes{}); err != nil {
		return err
	}

	if err := session.Shell(); err != nil {
		return err
	}

	return session.Wait()
}
