package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/charmbracelet/wishlist"
	"github.com/stretchr/testify/require"
)

func TestGetConfig(t *testing.T) {
	tmp := t.TempDir()
	dir, err := os.Getwd()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, os.Chdir(dir)) })

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
			_, err := getConfig(filepath.Join(dir, "testdata/valid"))
			require.NoError(t, err)
		})

		t.Run("invalid", func(t *testing.T) {
			_, err := getConfig(filepath.Join(dir, "testdata/invalid"))
			require.Error(t, err)
		})

		t.Run("not found", func(t *testing.T) {
			_, err := getConfig(filepath.Join(dir, "testdata/nope"))
			require.NoError(t, err)
		})
	})
}

func TestAllPossibleConfigPaths(t *testing.T) {
	cfg, err := os.UserConfigDir()
	require.NoError(t, err)

	home, err := os.UserHomeDir()
	require.NoError(t, err)

	paths, err := userConfigPaths()
	require.NoError(t, err)
	require.Len(t, paths, 8)
	require.Equal(t, []string{
		".wishlist/config.yaml",
		".wishlist/config.yml",
		".wishlist/config",
		filepath.Join(cfg, "wishlist.yaml"),
		filepath.Join(cfg, "wishlist.yml"),
		filepath.Join(cfg, "wishlist"),
		filepath.Join(home, ".ssh", "config"),
		"/etc/ssh/ssh_config",
	}, paths)
}
