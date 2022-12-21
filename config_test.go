package wishlist

import (
	"testing"

	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"
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
			env: map[string]string{
				"LC_ALL": "en_US.UTF-8",
				"LANG":   "en_US",
			},
		},
		{
			name: "some invalid env",
			endpoint: &Endpoint{
				SendEnv: []string{},
				SetEnv: []string{
					"FOO=foo",
					"BAR",
					"IGNR=",
					"=ignr",
				},
			},
			env: map[string]string{
				"FOO":    "foo",
				"IGNR":   "",
				"LC_ALL": "en_US.UTF-8",
				"LANG":   "en_US",
			},
		},
		{
			name: "some env",
			endpoint: &Endpoint{
				SendEnv: []string{
					"FOO_*",
				},
				SetEnv: []string{
					"FOO=foo",
					"BAR=bar",
				},
			},
			env: map[string]string{
				"BAR":     "bar",
				"FOO":     "foo",
				"LC_ALL":  "en_US.UTF-8",
				"FOO_BAR": "foobar",
				"LANG":    "en_US",
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.env, tt.endpoint.Environment(
				"LC_ALL=en_US.UTF-8",
				"LANG=en_US",
				"FOO_BAR=foobar",
			))
		})
	}
}
