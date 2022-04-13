package wishlist

import (
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"

	"github.com/charmbracelet/keygen"
	"github.com/charmbracelet/wishlist/home"
	"github.com/gliderlabs/ssh"
	gossh "golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/knownhosts"
	"golang.org/x/term"
)

var errNoRemoteAgent = fmt.Errorf("no agent forwarded")

// remoteBestAuthMethod returns an auth method.
//
// it first tries to use ssh-agent, if that's not available, it creates and uses a new key pair.
func remoteBestAuthMethod(s ssh.Session) (gossh.AuthMethod, agent.Agent, closers, error) {
	// TODO: we should probably make password protected keys work here too
	method, agt, cls, err := tryRemoteAuthAgent(s)
	if err != nil {
		return nil, nil, nil, err
	}
	if method != nil {
		return method, agt, cls, nil
	}

	method, err = tryNewKey()
	return method, nil, nil, err
}

// localBestAuthMethod figures out which authentication method is the best for
// the given endpoint.
//
// preference order:
// - the IdentityFiles, if they were set in the endpoint
// - the local ssh agent, if available
// - common key filenames under ~/.ssh/
//
// If any of the methods fails, it returns an error.
// It'll return a nil list if none of the methods is available.
func localBestAuthMethod(agt agent.Agent, e *Endpoint) ([]gossh.AuthMethod, error) {
	var methods []gossh.AuthMethod
	if method := agentAuthMethod(agt); method != nil {
		methods = append(methods, method)
	}

	if len(e.IdentityFiles) > 0 {
		ids, err := tryIdendityFiles(e)
		return append(methods, ids...), err
	}
	keys, err := tryUserKeys()
	return append(methods, keys...), err
}

// agentAuthMethod setups an auth method for the given agent.
func agentAuthMethod(agt agent.Agent) gossh.AuthMethod {
	if agt == nil {
		return nil
	}
	log.Println("using SSH agent")
	return gossh.PublicKeysCallback(agt.Signers)
}

// getLocalAgent checks if there's a local agent at $SSH_AUTH_SOCK and, if so,
// returns a connection to it through agent.Agent.
func getLocalAgent() (agent.Agent, closers, error) {
	socket := os.Getenv("SSH_AUTH_SOCK")
	if socket == "" {
		return nil, nil, nil
	}
	conn, err := net.Dial("unix", socket)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to connect to SSH_AUTH_SOCK: %w", err)
	}
	return agent.NewClient(conn), closers{conn.Close}, nil
}

func getRemoteAgent(s ssh.Session) (agent.Agent, closers, error) {
	_, _ = s.SendRequest("auth-agent-req@openssh.com", true, nil)
	if !ssh.AgentRequested(s) {
		return nil, nil, errNoRemoteAgent
	}

	l, err := ssh.NewAgentListener()
	if err != nil {
		return nil, nil, err // nolint:wrapcheck
	}
	go ssh.ForwardAgentConnections(l, s)

	conn, err := net.Dial(l.Addr().Network(), l.Addr().String())
	if err != nil {
		return nil, closers{l.Close}, err // nolint:wrapcheck
	}

	return agent.NewClient(conn), closers{l.Close, conn.Close}, nil
}

// tryRemoteAuthAgent will try to use an ssh-agent to authenticate.
func tryRemoteAuthAgent(s ssh.Session) (gossh.AuthMethod, agent.Agent, closers, error) {
	agent, closers, err := getRemoteAgent(s)
	if err != nil {
		if errors.Is(err, errNoRemoteAgent) {
			fmt.Fprintf(s.Stderr(), "wishlist: ssh agent not available\n\r")
			return nil, nil, closers, nil
		}
		return nil, nil, closers, err
	}

	return gossh.PublicKeysCallback(agent.Signers), agent, closers, nil
}

// tryNewKey will create a .wishlist/client_ed25519 keypair if one does not exist.
// It will return an auth method that uses the keypair if it exist or is successfully created.
func tryNewKey() (gossh.AuthMethod, error) {
	key, err := keygen.New(".wishlist/client", nil, keygen.Ed25519)
	if err != nil {
		return nil, err // nolint:wrapcheck
	}

	signer, err := gossh.ParsePrivateKey(key.PrivateKeyPEM())
	if err != nil {
		return nil, err // nolint:wrapcheck
	}

	if key.KeyPairExists() {
		return gossh.PublicKeys(signer), nil
	}

	return gossh.PublicKeys(signer), key.WriteKeys()
}

func tryIdendityFiles(e *Endpoint) ([]gossh.AuthMethod, error) {
	methods := make([]gossh.AuthMethod, 0, len(e.IdentityFiles))
	for _, id := range e.IdentityFiles {
		method, err := tryIdentityFile(id)
		if err != nil {
			return nil, err
		}
		methods = append(methods, method)
	}
	return methods, nil
}

// tryIdentityFile tries to use the given idendity file.
func tryIdentityFile(id string) (gossh.AuthMethod, error) {
	h, err := home.ExpandPath(id)
	if err != nil {
		return nil, err // nolint: wrapcheck
	}
	return parsePrivateKey(h, nil)
}

// tryUserKeys will try to find id_rsa and id_ed25519 keys in the user $HOME/~.ssh folder.
func tryUserKeys() ([]gossh.AuthMethod, error) {
	return tryUserKeysInternal(home.ExpandPath)
}

// https://github.com/openssh/openssh-portable/blob/8a0848cdd3b25c049332cd56034186b7853ae754/readconf.c#L2534-L2546
// https://github.com/openssh/openssh-portable/blob/2dc328023f60212cd29504fc05d849133ae47355/pathnames.h#L71-L81
// TODO: ideally the password should be asked only on use, in order to
//   avoid asking the password of all user keys to use only one of them.
func tryUserKeysInternal(pathResolver func(string) (string, error)) ([]gossh.AuthMethod, error) {
	var methods []gossh.AuthMethod // nolint: prealloc
	for _, name := range []string{
		"id_rsa",
		// "id_dsa", // unhandled by go, deprecated by openssh
		"id_ecdsa",
		"id_ecdsa_sk",
		"id_ed25519",
		"id_ed25519_sk",
		// "id_xmss", // unhandled by go - and most openssh versions it seems
	} {
		path, err := pathResolver(filepath.Join("~/.ssh", name))
		if err != nil {
			return nil, err
		}
		method, err := parsePrivateKey(path, nil)
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
		signer, err = gossh.ParsePrivateKeyWithPassphrase(bts, password)
	}
	if err != nil {
		pwderr := &gossh.PassphraseMissingError{}
		if errors.As(err, &pwderr) {
			fmt.Printf("Enter the password for %q: ", path)
			password, err := term.ReadPassword(int(os.Stdin.Fd()))
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
