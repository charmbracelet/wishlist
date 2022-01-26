package wishlist

import (
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/keygen"
	"github.com/gliderlabs/ssh"
	gossh "golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/knownhosts"
	"golang.org/x/term"
)

// remoteBestAuthMethod returns an auth method.
//
// it first tries to use ssh-agent, if that's not available, it creates and uses a new key pair.
func remoteBestAuthMethod(s ssh.Session) (gossh.AuthMethod, closers, error) {
	method, closers, err := tryRemoteAuthAgent(s)
	if err != nil {
		return method, closers, err
	}
	if method != nil {
		return method, closers, nil
	}

	method, err = tryNewKey()
	return method, closers, err
}

// localBestAuthMethod figures out which authentication method is the best for
// the given endpoint.
//
// preference order:
// - an IdentityFile, if there's one set in the endpoint
// - the local ssh agent, if available
// - common key filenames under ~/.ssh/
//
// If any of the methods fails, it returns an error.
// It'll return a nil list if none of the methods is available.
func localBestAuthMethod(e *Endpoint) ([]gossh.AuthMethod, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get user home dir: %w", err)
	}
	if e.IdentityFile != "" {
		return toAuthMethodList(tryIdentityFile(home, e.IdentityFile))
	}
	if method, err := tryLocalAgent(); err != nil || method != nil {
		return toAuthMethodList(method, err)
	}
	return tryUserKeys(home)
}

func toAuthMethodList(m gossh.AuthMethod, err error) ([]gossh.AuthMethod, error) {
	return []gossh.AuthMethod{m}, err
}

// tryLocalAgent checks if there's a local agent at $SSH_AUTH_SOCK and, if so,
// uses it to authenticate.
func tryLocalAgent() (gossh.AuthMethod, error) {
	socket := os.Getenv("SSH_AUTH_SOCK")
	if socket == "" {
		return nil, nil
	}
	conn, err := net.Dial("unix", socket)
	if err != nil {
		return nil, fmt.Errorf("failed to connecto to SSH_AUTH_SOCK: %w", err)
	}
	log.Println("using SSH agent")
	return gossh.PublicKeysCallback(agent.NewClient(conn).Signers), nil
}

// tryRemoteAuthAgent will try to use an ssh-agent to authenticate.
func tryRemoteAuthAgent(s ssh.Session) (gossh.AuthMethod, closers, error) {
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

// tryIdentityFile tries to use the given idendity file.
func tryIdentityFile(home, id string) (gossh.AuthMethod, error) {
	return parsePrivateKey(expand(home, id), nil)
}

func expand(home, p string) string {
	if !strings.HasPrefix(p, "~/") {
		return p
	}
	return filepath.Join(home, strings.TrimPrefix(p, "~/"))

}

// tryUserKeys will try to find id_rsa and id_ed25519 keys in the user $HOME/~.ssh folder.
func tryUserKeys(home string) ([]gossh.AuthMethod, error) {
	var methods []gossh.AuthMethod // nolint: prealloc
	for _, name := range []string{
		"id_rsa",
		"id_ed25519",
	} {
		method, err := parsePrivateKey(filepath.Join(home, ".ssh", name), nil)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return methods, err
		}
		methods = append(methods, method)
	}
	return methods, nil
}

func parsePrivateKey(path string, password []byte) (gossh.AuthMethod, error) {
	bts, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read key: %q: %w", path, err)
	}
	var signer gossh.Signer
	if len(password) == 0 {
		signer, err = gossh.ParsePrivateKey(bts)
	} else {
		signer, err = gossh.ParsePrivateKeyWithPassphrase(bts, []byte(password))
	}
	if err != nil {
		pwderr := &gossh.PassphraseMissingError{}
		if errors.As(err, &pwderr) {
			fmt.Printf("Enter the password for %q: ", path) // TODO: why is this not displayed?
			password, err = term.ReadPassword(int(os.Stdin.Fd()))
			fmt.Println()
			if err != nil {
				return nil, fmt.Errorf("failed to read password: %q", err)
			}
			return parsePrivateKey(path, password)
		}
		return nil, fmt.Errorf("failed to parse private key: %q: %w", path, err)
	}
	log.Printf("using %q", path)
	return gossh.PublicKeys(signer), nil
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
