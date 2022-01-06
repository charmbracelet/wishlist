package wishlist

import (
	"testing"

	"github.com/charmbracelet/wish"
	"github.com/gliderlabs/ssh"
	"github.com/stretchr/testify/require"
)

func TestEndpointValid(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		require.True(t, Endpoint{
			Name:    "test",
			Address: "test",
		}.Valid())
		require.True(t, Endpoint{
			Name: "test",
			Middlewares: []wish.Middleware{
				func(h ssh.Handler) ssh.Handler { return h },
			},
		}.Valid())
	})

	t.Run("no name", func(t *testing.T) {
		require.False(t, Endpoint{
			Name:    "",
			Address: "test",
		}.Valid())
	})

	t.Run("no address or middleware", func(t *testing.T) {
		require.False(t, Endpoint{
			Name: "test",
		}.Valid())
	})
}

func TestShoudListen(t *testing.T) {
	t.Run("true", func(t *testing.T) {
		require.True(t, Endpoint{
			Name: "test",
			Middlewares: []wish.Middleware{
				func(h ssh.Handler) ssh.Handler { return h },
			},
		}.ShouldListen())
	})

	t.Run("false", func(t *testing.T) {
		require.False(t, Endpoint{
			Name: "test",
		}.ShouldListen())
	})
}
