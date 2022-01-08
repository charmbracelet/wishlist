package sshconfig

import (
	"fmt"
	"testing"
	"testing/iotest"

	"github.com/charmbracelet/wishlist"
	"github.com/stretchr/testify/require"
)

func TestParseFile(t *testing.T) {
	t.Run("good", func(t *testing.T) {
		endpoints, err := ParseFile("testdata/good.ssh_config")
		require.NoError(t, err)

		require.ElementsMatch(t, []*wishlist.Endpoint{
			{
				Name:    "darkstar",
				Address: "darkstar.local:22",
			},
			{
				Name:    "supernova",
				Address: "supernova.local:22",
				User:    "notme",
			},
			{
				Name:    "app1",
				Address: "app.foo.local:2222",
			},
			{
				Name:    "app2",
				Address: "app.foo.local:2223",
				User:    "someoneelse",
			},
			{
				Name:    "multiple1",
				Address: "multi1.foo.local:22",
				User:    "multi",
			},
			{
				Name:    "multiple2",
				Address: "multi2.foo.local:2223",
				User:    "multi",
			},
			{
				Name:    "multiple3",
				Address: "multi3.foo.local:22",
				User:    "overridden",
			},
		}, endpoints)
	})

	t.Run("invalid node", func(t *testing.T) {
		endpoints, err := ParseFile("testdata/invalid_node.ssh_config")
		require.Empty(t, endpoints)
		require.EqualError(t, err, `invalid node on app "invalid": "HostNameinvalid-because-no-spaces"`)
	})

	t.Run("invalid path", func(t *testing.T) {
		endpoints, err := ParseFile("testdata/nope.ssh_config")
		require.Empty(t, endpoints)
		require.EqualError(t, err, `failed to open config: open testdata/nope.ssh_config: no such file or directory`)
	})
}

func TestParseReader(t *testing.T) {
	t.Run("error", func(t *testing.T) {
		endpoints, err := ParseReader(iotest.ErrReader(fmt.Errorf("any")))
		require.EqualError(t, err, "failed to read config: any")
		require.Empty(t, endpoints)
	})
}
