package wishlist

import (
	"context"
	"io"
	"net"
	"strconv"
	"sync"
	"testing"

	"github.com/gliderlabs/ssh"
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

func TestPublicKeyHandler(t *testing.T) {
	t.Run("no users", func(t *testing.T) {
		require.Nil(t, publicKeyAccessOption([]User{}))
	})

	t.Run("with users", func(t *testing.T) {
		pubkey := "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIMYKQ6pT3+iZBROfFKKT/4GVc1Xws776bE67cF3zUQPS foo@bar"
		pubkey2 := "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIDfBMpbghW82c1zk9LauP7G/LqXtTeQrU6Do9FUY1FJ5 foo@bar"
		k, _, _, _, err := ssh.ParseAuthorizedKey([]byte(pubkey))
		require.NoError(t, err)

		t.Run("authorized", func(t *testing.T) {
			require.True(t, publicKeyAccessOption([]User{
				{
					Name:       "test",
					PublicKeys: []string{pubkey},
				},
			})(fakeCtx{}, k))
		})

		t.Run("unauthorized wrong username", func(t *testing.T) {
			require.False(t, publicKeyAccessOption([]User{
				{
					Name:       "not-test",
					PublicKeys: []string{pubkey},
				},
			})(fakeCtx{}, k))
		})

		t.Run("unauthorized wrong key", func(t *testing.T) {
			require.False(t, publicKeyAccessOption([]User{
				{
					Name:       "test",
					PublicKeys: []string{pubkey2},
				},
			})(fakeCtx{}, k))
		})

		t.Run("invalid key", func(t *testing.T) {
			require.False(t, publicKeyAccessOption([]User{
				{
					Name:       "test",
					PublicKeys: []string{"giberrish"},
				},
			})(fakeCtx{}, k))
		})
	})
}

type fakeCtx struct {
	context.Context
	sync.Locker
}

func (ctx fakeCtx) User() string {
	return "test"
}

func (ctx fakeCtx) SessionID() string {
	return "test"
}

func (ctx fakeCtx) ClientVersion() string {
	return "1.0"
}

func (ctx fakeCtx) ServerVersion() string {
	return "1.0"
}

func (ctx fakeCtx) RemoteAddr() net.Addr {
	return &net.IPAddr{}
}

func (ctx fakeCtx) LocalAddr() net.Addr {
	return &net.IPAddr{}
}

func (ctx fakeCtx) Permissions() *ssh.Permissions {
	return nil
}

func (ctx fakeCtx) SetValue(key, value interface{}) {}

func portFromAddr(tb testing.TB, addr string) int64 {
	tb.Helper()
	_, port, err := net.SplitHostPort(addr)
	require.NoError(tb, err)

	result, err := strconv.ParseInt(port, 10, 64)
	require.NoError(tb, err)
	return result
}
