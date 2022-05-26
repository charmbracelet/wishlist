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

func TestEnvironment(t *testing.T) {
	type testcase struct {
		name     string
		env      map[string]string
		endpoint *Endpoint
	}
	for _, tt := range []testcase{
		{
			name:     "no env",
			endpoint: &Endpoint{},
			env:      map[string]string{},
		},
		{
			name: "some invalid env",
			endpoint: &Endpoint{
				SendEnv: []string{
					"FOO",
					"BAR",
					"NOPE",
				},
				SetEnv: []string{
					"FOO=foo",
					"BAR",
					"IGNR=",
				},
			},
			env: map[string]string{
				"FOO":  "foo",
				"IGNR": "",
			},
		},
		{
			name: "some env",
			endpoint: &Endpoint{
				SendEnv: []string{
					"FOO",
					"BAR",
					"NOPE",
					"LC_*",
				},
				SetEnv: []string{
					"FOO=foo",
					"BAR=bar",
				},
			},
			env: map[string]string{
				"BAR":    "bar",
				"FOO":    "foo",
				"LC_ALL": "en_US.UTF-8",
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.env, tt.endpoint.Environment("LC_ALL=en_US.UTF-8"))
		})
	}
}
