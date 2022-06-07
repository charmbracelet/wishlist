package home

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExpandPath(t *testing.T) {
	t.Run("expand", func(t *testing.T) {
		path, err := ExpandPath("~/.ssh/foo")
		require.NoError(t, err)

		home, err := os.UserHomeDir()
		require.NoError(t, err)

		require.Equal(t, filepath.Join(home, ".ssh/foo"), path)
	})

	t.Run("noexpand", func(t *testing.T) {
		p := "/home/john/.ssh/foo"
		path, err := ExpandPath(p)
		require.NoError(t, err)
		require.Equal(t, p, path)
	})
}
