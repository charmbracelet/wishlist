package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/charmbracelet/wishlist"
	"github.com/stretchr/testify/require"
)

func TestGetConfig(t *testing.T) {
	dir, err := os.Getwd()
	require.NoError(t, err)

	tmp := t.TempDir()
	require.NoError(t, os.Chdir(tmp))
	require.NoError(t, os.Mkdir(".wishlist", 0o755))
	require.NoError(t, os.WriteFile(".wishlist/config.yaml", []byte(`
# just a valid yaml file to default to
`), 0o644))

	t.Run("yaml", func(t *testing.T) {
		t.Run("valid", func(t *testing.T) {
			cfg, err := getConfig(filepath.Join(dir, "testdata/valid.yaml"))
			require.NoError(t, err)
			require.Equal(t, wishlist.Config{Listen: "127.0.0.1"}, cfg)
		})

		t.Run("invalid", func(t *testing.T) {
			_, err := getConfig(filepath.Join(dir, "testdata/invalid.yaml"))
			require.Error(t, err)
		})

		t.Run("not found", func(t *testing.T) {
			_, err := getConfig(filepath.Join(dir, "testdata/nope.yaml"))
			require.NoError(t, err)
		})
	})

	t.Run("ssh", func(t *testing.T) {
		t.Run("valid", func(t *testing.T) {
			_, err := getConfig(filepath.Join(dir, "testdata/valid.ssh_config"))
			require.NoError(t, err)
		})

		t.Run("invalid", func(t *testing.T) {
			_, err := getConfig(filepath.Join(dir, "testdata/invalid.ssh_config"))
			require.Error(t, err)
		})

		t.Run("not found", func(t *testing.T) {
			_, err := getConfig(filepath.Join(dir, "testdata/nope.ssh_config"))
			require.NoError(t, err)
		})
	})
}
