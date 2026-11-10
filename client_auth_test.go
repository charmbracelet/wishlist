package wishlist

import (
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/keygen"
	"github.com/stretchr/testify/require"
	gossh "golang.org/x/crypto/ssh"
)

func TestUserKeys(t *testing.T) {
	fn := func(home string) func(string) (string, error) {
		return func(s string) (string, error) {
			return filepath.Join(home, strings.TrimPrefix(s, "~"+string(os.PathSeparator))), nil
		}
	}

	sshKeygen := func(tb testing.TB, tmp string, algo keygen.KeyType) {
		tb.Helper()
		path := filepath.Join(tmp, ".ssh")
		require.NoError(tb, os.MkdirAll(path, 0o765))
		_, err := keygen.New(path+"/id_"+algo.String(), keygen.WithKeyType(algo), keygen.WithWrite())
		require.NoError(tb, err)
	}

	t.Run("rsa", func(t *testing.T) {
		tmp := t.TempDir()
		sshKeygen(t, tmp, keygen.RSA)
		methods, err := tryUserKeysInternal(fn(tmp))
		require.NoError(t, err)
		require.Len(t, methods, 1)
	})

	t.Run("ecdsa", func(t *testing.T) {
		tmp := t.TempDir()
		sshKeygen(t, tmp, keygen.ECDSA)
		methods, err := tryUserKeysInternal(fn(tmp))
		require.NoError(t, err)
		require.Len(t, methods, 1)
	})

	t.Run("ed25519", func(t *testing.T) {
		tmp := t.TempDir()
		sshKeygen(t, tmp, keygen.Ed25519)
		methods, err := tryUserKeysInternal(fn(tmp))
		require.NoError(t, err)
		require.Len(t, methods, 1)
	})

	// TODO: how to test ecdsa-sk and ed25519-sk?
}

func TestHostKeyCallbackAcceptsOpenSSHHostCertificateCA(t *testing.T) {
	_, caPrivate, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	caSigner, err := gossh.NewSignerFromKey(caPrivate)
	require.NoError(t, err)

	_, hostPrivate, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	hostSigner, err := gossh.NewSignerFromKey(hostPrivate)
	require.NoError(t, err)

	cert := &gossh.Certificate{
		Key:             hostSigner.PublicKey(),
		CertType:        gossh.HostCert,
		ValidPrincipals: []string{"host1.example-tailnet.ts.net"},
		ValidAfter:      0,
		ValidBefore:     gossh.CertTimeInfinity,
		SignatureKey:    caSigner.PublicKey(),
	}
	require.NoError(t, cert.SignCert(rand.Reader, caSigner))
	certSigner, err := gossh.NewCertSigner(cert, hostSigner)
	require.NoError(t, err)

	knownHosts := filepath.Join(t.TempDir(), "known_hosts")
	caLine := fmt.Sprintf("@cert-authority *.example-tailnet.ts.net %s\n", string(gossh.MarshalAuthorizedKey(caSigner.PublicKey())))
	require.NoError(t, os.WriteFile(knownHosts, []byte(caLine), 0o600))

	for _, port := range []int{22, 2222} {
		t.Run(fmt.Sprintf("port-%d", port), func(t *testing.T) {
			address := fmt.Sprintf("host1.example-tailnet.ts.net:%d", port)
			callback := hostKeyCallback(&Endpoint{Address: address}, knownHosts)
			callbackErr := callback(
				address,
				&net.TCPAddr{IP: net.ParseIP("192.0.2.1"), Port: port},
				certSigner.PublicKey(),
			)
			require.NoError(t, callbackErr)
		})
	}
}
