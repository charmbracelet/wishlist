package wishlist

import (
	"io"
	"net"
	"strconv"
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

func TestGetFirstOpenPort(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = l.Close() })
	open := portFromAddr(t, l.Addr().String())
	require.NoError(t, l.Close())

	l2, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, l2.Close()) })
	closed := portFromAddr(t, l2.Addr().String())

	t.Log("open:", open)
	t.Log("closed:", closed)
	t.Run("gets port", func(t *testing.T) {
		port, err := getFirstOpenPort("127.0.0.1", closed, open)
		require.NoError(t, err)
		require.Equal(t, open, port)
	})

	t.Run("only used ports", func(t *testing.T) {
		port, err := getFirstOpenPort("127.0.0.1", closed)
		require.Error(t, err)
		require.Equal(t, int64(0), port)
	})
}

func portFromAddr(tb testing.TB, addr string) int64 {
	tb.Helper()
	_, port, err := net.SplitHostPort(addr)
	require.NoError(tb, err)

	result, err := strconv.ParseInt(port, 10, 64)
	require.NoError(tb, err)
	return result
}
