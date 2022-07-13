package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/charmbracelet/wishlist"
	"github.com/stretchr/testify/require"
)

func TestParseExampleYaml(t *testing.T) {
	cfg, err := getConfig("../../_example/config.yaml")
	require.NoError(t, err)
	require.Equal(t, "127.0.0.1", cfg.Listen)
	require.Equal(t, int64(2223), cfg.Port)
	require.Len(t, cfg.Endpoints, 1)
	require.Equal(t, wishlist.Endpoint{
		Name:    "appname",
		Address: "foo.local:2234",
		Link: wishlist.Link{
			Name: "Optional link name",
			URL:  "https://github.com/charmbracelet/wishlist",
		},
		Desc:          "A description of this endpoint.\nCan have multiple lines.",
		User:          "notme",
		RemoteCommand: "uptime -a",
		ForwardAgent:  true,
		IdentityFiles: []string{"~/.ssh/id_rsa", "~/.ssh/id_ed25519"},
		RequestTTY:    true,
		SetEnv:        []string{"FOO=bar", "BAR=baz"},
		SendEnv:       []string{"LC_*", "LANG", "SOME_ENV"},
	}, *cfg.Endpoints[0])
	require.Len(t, cfg.Users, 1)
	require.Equal(t, wishlist.User{
		Name: "carlos",
		PublicKeys: []string{
			"ssh-rsa AAAAB3Nz...",
			"ssh-ed25519 AAAA...",
		},
	}, cfg.Users[0])
}

func TestParseExampleSSHConfig(t *testing.T) {
	cfg, err := getConfig("../../_example/config")
	require.NoError(t, err)
	require.NoError(t, err)
	require.Empty(t, cfg.Listen)
	require.Empty(t, cfg.Port)
	require.Len(t, cfg.Endpoints, 2)
	require.Equal(t, wishlist.Endpoint{
		Name:          "foo",
		Address:       "foo.bar:2223",
		User:          "notme",
		IdentityFiles: []string{"~/.ssh/foo_ed25519"},
		ForwardAgent:  true,
		RequestTTY:    true,
		RemoteCommand: "tmux a",
		SendEnv:       []string{"FOO_*", "BAR_*"},
		SetEnv:        []string{"HELLO=world", "BYE=world"},
	}, *cfg.Endpoints[0])
	require.Equal(t, wishlist.Endpoint{
		Name:    "ssh.example.com",
		Address: "ssh.example.com:22",
	}, *cfg.Endpoints[1])
}

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

func TestUserConfigPaths(t *testing.T) {
	t.Run("all", func(t *testing.T) {
		cfg, err := os.UserConfigDir()
		require.NoError(t, err)

		home, err := os.UserHomeDir()
		require.NoError(t, err)
		paths := userConfigPaths()
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
	})

	t.Run("no config dir", func(t *testing.T) {
		_ = os.Unsetenv("XDG_CONFIG_HOME")
		_ = os.Unsetenv("HOME")

		paths := userConfigPaths()
		require.Len(t, paths, 4)
		require.Equal(t, []string{
			".wishlist/config.yaml",
			".wishlist/config.yml",
			".wishlist/config",
			"/etc/ssh/ssh_config",
		}, paths)
	})
}
