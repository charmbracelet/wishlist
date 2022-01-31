package wishlist

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUserKeys(t *testing.T) {
	fn := func(home string) func(string) (string, error) {
		return func(s string) (string, error) {
			return filepath.Join(home, strings.TrimPrefix(s, "~/")), nil
		}
	}

	sshKeygen := func(tb testing.TB, cwd string, args ...string) {
		tb.Helper()
		cmd := exec.Command("ssh-keygen", args...)
		cmd.Dir = filepath.Join(cwd, ".ssh")
		require.NoError(tb, os.MkdirAll(cmd.Dir, 0766))
		out, err := cmd.CombinedOutput()
		require.NoError(t, err)
		t.Log(string(out))
	}

	t.Run("rsa", func(t *testing.T) {
		tmp := t.TempDir()

		sshKeygen(t, tmp, "-t", "rsa", "-f", "./id_rsa", "-N", "", "-q")
		methods, err := tryUserKeysInternal(fn(tmp))
		require.NoError(t, err)
		require.Len(t, methods, 1)
	})

	t.Run("ecdsa", func(t *testing.T) {
		tmp := t.TempDir()

		sshKeygen(t, tmp, "-t", "ecdsa", "-f", "./id_rsa", "-N", "", "-q")
		methods, err := tryUserKeysInternal(fn(tmp))
		require.NoError(t, err)
		require.Len(t, methods, 1)
	})

	t.Run("ed25519", func(t *testing.T) {
		tmp := t.TempDir()

		sshKeygen(t, tmp, "-t", "ed25519", "-f", "./id_rsa", "-N", "", "-q")
		methods, err := tryUserKeysInternal(fn(tmp))
		require.NoError(t, err)
		require.Len(t, methods, 1)
	})

	// TODO: how to test ecdsa-sk and ed25519-sk?
}
