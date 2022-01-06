package wishlist

import (
	"io"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestToAddress(t *testing.T) {
	require.Equal(t, "test:1234", toAddress("test", 1234))
}

func TestCloseAll(t *testing.T) {
	t.Run("no err", func(t *testing.T) {
		require.NoError(t, closeAll([]func() error{
			func() error { return nil },
			func() error { return nil },
		}))
	})

	t.Run("error", func(t *testing.T) {
		require.ErrorAs(t, closeAll([]func() error{
			func() error { return nil },
			func() error { return io.ErrClosedPipe },
			func() error { return nil },
		}), &io.ErrClosedPipe)
	})

	t.Run("multierror", func(t *testing.T) {
		err := closeAll([]func() error{
			func() error { return io.ErrClosedPipe },
			func() error { return io.EOF },
		})
		require.ErrorAs(t, err, &io.ErrClosedPipe)
		require.ErrorAs(t, err, &io.EOF)
	})
}
