package wishlist

import (
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"

	"github.com/charmbracelet/keygen"
	"github.com/gliderlabs/ssh"
	gossh "golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/knownhosts"
)

// remoteBestAuthMethod returns an auth method.
//
// it first tries to use ssh-agent, if that's not available, it creates and uses a new key pair.
func remoteBestAuthMethod(s ssh.Session) (gossh.AuthMethod, closers, error) {
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
			return nil, nil, err // nolint:wrapcheck
		}
		go ssh.ForwardAgentConnections(l, s)

		conn, err := net.Dial(l.Addr().Network(), l.Addr().String())
		if err != nil {
			return nil, closers{l.Close}, err // nolint:wrapcheck
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
		return nil, err // nolint:wrapcheck
	}

	signer, err := gossh.ParsePrivateKey(key.PrivateKeyPEM)
	if err != nil {
		return nil, err // nolint:wrapcheck
	}

	if key.IsKeyPairExists() {
		return gossh.PublicKeys(signer), nil
	}

	return gossh.PublicKeys(signer), key.WriteKeys()
}

// tryUserKeys will try to find id_rsa and id_ed25519 keys in the user $HOME/~.ssh folder.
// TODO: parse ssh config and get keys from there if any
func tryUserKeys(home string) ([]gossh.AuthMethod, error) {
	var methods []gossh.AuthMethod
	for _, name := range []string{
		"id_rsa",
		"id_ed25519",
	} {
		path := filepath.Join(home, ".ssh", name)
		bts, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		signer, err := gossh.ParsePrivateKey(bts)
		if err != nil {
			return methods, err
		}
		log.Printf("using %q", path)
		methods = append(methods, gossh.PublicKeys(signer))
	}

	return methods, nil
}

// hostKeyCallback returns a callback that will be used to verify the host key.
//
// it creates a file in the given path, and uses that to verify hosts and keys.
// if the host does not exist there, it adds it so its available next time, as plain old `ssh` does.
func hostKeyCallback(e *Endpoint, path string) gossh.HostKeyCallback {
	return func(hostname string, remote net.Addr, key gossh.PublicKey) error {
		kh, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600) // nolint:gomnd
		if err != nil {
			return fmt.Errorf("failed to open known_hosts: %w", err)
		}
		defer func() { _ = kh.Close() }()

		callback, err := knownhosts.New(kh.Name())
		if err != nil {
			return fmt.Errorf("failed to check known_hosts: %w", err)
		}

		if err := callback(hostname, remote, key); err != nil {
			var kerr *knownhosts.KeyError
			if errors.As(err, &kerr) {
				if len(kerr.Want) > 0 {
					return fmt.Errorf("possible man-in-the-middle attack: %w", err)
				}
				// if want is empty, it means the host was not in the known_hosts file, so lets add it there.
				fmt.Fprintln(kh, knownhosts.Line([]string{e.Address}, key))
				return nil
			}
			return fmt.Errorf("failed to check known_hosts: %w", err)
		}
		return nil
	}
}
