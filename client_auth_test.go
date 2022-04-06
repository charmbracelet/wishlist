package wishlist

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/keygen"
	"github.com/stretchr/testify/require"
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
		_, err := keygen.NewWithWrite(path+"/id", nil, keygen.RSA)
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
