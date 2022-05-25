package sshconfig

import (
	"fmt"
	"os"
	"testing"
	"testing/iotest"

	"github.com/charmbracelet/wishlist"
	"github.com/stretchr/testify/require"
)

func TestParseFile(t *testing.T) {
	t.Run("good", func(t *testing.T) {
		endpoints, err := ParseFile("testdata/good")
		require.NoError(t, err)

		require.Len(t, endpoints, 9)
		require.Equal(t, []*wishlist.Endpoint{
			{
				Name:    "darkstar",
				Address: "darkstar.local:22",
			},
			{
				Name:    "supernova",
				Address: "supernova.local:22",
				User:    "notme",
				Environment: []string{
					"FOO=foo",
					"BAR=bar",
				},
			},
			{
				Name:    "app1",
				Address: "app.foo.local:2222",
			},
			{
				Name:          "app2",
				Address:       "app.foo.local:2223",
				User:          "someoneelse",
				IdentityFiles: []string{"./testdata/key"},
				ForwardAgent:  true,
			},
			{
				Name:        "multiple1",
				Address:     "multi1.foo.local:22",
				User:        "multi",
				Environment: []string{"FOO=foobar"},
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
			{
				Name:         "no.hostname",
				Address:      "no.hostname:23231",
				ForwardAgent: true,
			},
			{
				Name:    "only.host",
				Address: "only.host:22",
			},
		}, endpoints)
	})

	t.Run("invalid node", func(t *testing.T) {
		endpoints, err := ParseFile("testdata/invalid_node")
		require.Empty(t, endpoints)
		require.EqualError(t, err, `invalid node on app "invalid": "HostNameinvalid-because-no-spaces"`)
	})

	t.Run("invalid path", func(t *testing.T) {
		endpoints, err := ParseFile("testdata/nope")
		require.Empty(t, endpoints)
		require.ErrorIs(t, err, os.ErrNotExist)
	})
}

func TestParseReader(t *testing.T) {
	t.Run("error", func(t *testing.T) {
		endpoints, err := ParseReader(iotest.ErrReader(fmt.Errorf("any")))
		require.EqualError(t, err, "failed to read config: any")
		require.Empty(t, endpoints)
	})
}

func TestParseIncludes(t *testing.T) {
	endpoints, err := ParseFile("testdata/include")
	require.NoError(t, err)
	require.ElementsMatch(t, []*wishlist.Endpoint{
		{
			Name:          "test.foo.bar",
			Address:       "test.foo.bar:2222",
			User:          "ciclano",
			IdentityFiles: []string{"~/.ssh/id_rsa2", "~/.ssh/other_id"},
		},
		{
			Name:    "something.else",
			Address: "something.else:2323",
			User:    "ciclano",
		},
	}, endpoints)
}

func TestMergeMaps(t *testing.T) {
	require.Equal(
		t,
		map[string]hostinfo{
			"foo": {
				Hostname:      "foo.bar",
				User:          "me",
				IdentityFiles: []string{"id_rsa", "id_ed25519"},
				Port:          "2321",
				RequestTTY:    "yes",
				RemoteCommand: "tmux new -A -s default",
			},
			"bar": {
				User: "yoda",
			},
			"foobar": {
				User:          "notme",
				Hostname:      "foobar.foo",
				IdentityFiles: []string{"id_ed25519"},
			},
		},
		merge(
			newHostinfoMapFrom(
				map[string]hostinfo{
					"foo": {
						Hostname:      "foo.bar",
						IdentityFiles: []string{"id_ed25519"},
						RequestTTY:    "yes",
					},
					"bar": {
						User: "yoda",
					},
				},
			),
			newHostinfoMapFrom(
				map[string]hostinfo{
					"foo": {
						User:          "me",
						IdentityFiles: []string{"id_rsa"},
						Port:          "2321",
						RemoteCommand: "tmux new -A -s default",
					},
					"foobar": {
						User:          "notme",
						Hostname:      "foobar.foo",
						IdentityFiles: []string{"id_ed25519"},
					},
				},
			),
		).inner,
	)
}

func TestSplit(t *testing.T) {
	wildcards, hosts := split(newHostinfoMapFrom(map[string]hostinfo{
		"*.foo.bar": {User: "yoda"},
		"*":         {Hostname: "foobar"},
		"foo.bar":   {User: "john"},
	},
	))

	require.Equal(t, 2, wildcards.length())
	require.Equal(t, map[string]hostinfo{
		"*.foo.bar": {User: "yoda"},
		"*":         {Hostname: "foobar"},
	}, wildcards.inner)

	require.Equal(t, 1, hosts.length())
	require.Equal(t, map[string]hostinfo{
		"foo.bar": {User: "john"},
	}, hosts.inner)
}

func TestHostinfoMap(t *testing.T) {
	m := newHostinfoMap()
	m.set("a", hostinfo{
		User: "a",
	})
	m.set("b", hostinfo{
		User: "bbbbb",
	})
	m.set("b", hostinfo{
		User: "b",
	})

	a, ok := m.get("a")
	require.True(t, ok)
	require.Equal(t, hostinfo{User: "a"}, a)

	b, ok := m.get("b")
	require.True(t, ok)
	require.Equal(t, hostinfo{User: "b"}, b)

	require.Equal(t, len(m.inner), m.length())
	require.Equal(t, len(m.keys), m.length())

	order := make([]string, 0, m.length())
	require.NoError(t, m.forEach(func(s string, _ hostinfo, _ error) error {
		order = append(order, s)
		return nil
	}))

	require.Equal(t, []string{"a", "b"}, order)

	expectedErr := fmt.Errorf("some error")
	require.Equal(t, expectedErr, m.forEach(func(s string, h hostinfo, e error) error {
		if e != nil {
			return e
		}
		if s == "b" {
			return expectedErr
		}
		return nil
	}))
}

func newHostinfoMapFrom(input map[string]hostinfo) *hostinfoMap {
	m := newHostinfoMap()
	for k, v := range input {
		m.set(k, v)
	}
	return m
}
