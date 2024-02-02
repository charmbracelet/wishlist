package sshconfig

import (
	_ "embed"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/iotest"
	"time"

	"github.com/charmbracelet/wishlist"
	"github.com/stretchr/testify/require"
)

func TestParseFile(t *testing.T) {
	t.Run("good", func(t *testing.T) {
		endpoints, err := ParseFile("testdata/good", nil)
		require.NoError(t, err)

		require.Len(t, endpoints, 11)

		results := map[string]*wishlist.Endpoint{
			"darkstar": {
				Name:    "darkstar",
				Address: "darkstar.local:22",
			},
			"supernova": {
				Name:    "supernova",
				Address: "supernova.local:22",
				User:    "notme",
				Timeout: 20 * time.Second,
				PreferredAuthentications: []string{
					"password",
					"keyboard-interactive",
					"publickey",
					"hostbased",
				},
				ProxyJump: "user@host:port",
				SendEnv: []string{
					"FOO",
				},
				SetEnv: []string{
					"BAR=bar",
				},
			},
			"app1": {
				Name:    "app1",
				Address: "app.foo.local:2222",
			},
			"app2": {
				Name:          "app2",
				Address:       "app.foo.local:2223",
				User:          "someoneelse",
				IdentityFiles: []string{"./testdata/key"},
				ForwardAgent:  true,
			},
			"multiple1": {
				Name:    "multiple1",
				Address: "multi1.foo.local:22",
				User:    "multi",
				Timeout: time.Second * 12,
				SendEnv: []string{
					"FOO",
				},
				SetEnv: []string{
					"FOOS=foobar",
				},
			},
			"multiple2": {
				Name:    "multiple2",
				Address: "multi2.foo.local:2223",
				User:    "multi",
				Timeout: time.Second * 12,
				SendEnv: []string{
					"FOO",
				},
				SetEnv: []string{
					"FOOS=foobar",
					"FOO2=foobar within quotes",
				},
			},
			"multiple3": {
				Name:    "multiple3",
				Address: "multi3.foo.local:22",
				User:    "overridden",
				Timeout: time.Second * 12,
				SendEnv: []string{
					"FOO",
					"AAA",
				},
			},
			"no.hostname": {
				Name:         "no.hostname",
				Address:      "no.hostname:23231",
				ForwardAgent: true,
			},
			"only.host": {
				Name:    "only.host",
				Address: "only.host:22",
			},

			"req.tty": {
				Name:       "req.tty",
				Address:    "req.tty:22",
				RequestTTY: true,
			},

			"remote.cmd": {
				Name:          "remote.cmd",
				Address:       "remote.cmd:22",
				RemoteCommand: "tmux",
			},
		}

		for _, e := range endpoints {
			t.Run(e.Name, func(t *testing.T) {
				require.Equal(t, results[e.Name], e)
			})
		}
	})

	t.Run("invalid node", func(t *testing.T) {
		endpoints, err := ParseFile("testdata/invalid_node", nil)
		require.Empty(t, endpoints)
		require.EqualError(t, err, `invalid node on app "invalid": "HostNameinvalid-because-no-spaces"`)
	})

	t.Run("invalid path", func(t *testing.T) {
		endpoints, err := ParseFile("testdata/nope", nil)
		require.Empty(t, endpoints)
		require.ErrorIs(t, err, os.ErrNotExist)
	})

	t.Run("with seed", func(t *testing.T) {
		endpoints, err := ParseReader(newNamedReader(`
Host *.local
  User carlos

Host zap.local
  ForwardAgent yes
		`, t.TempDir()), []*wishlist.Endpoint{
			{
				Name:    "foo.local",
				Address: "foo.local:22",
			},
			{
				Name:    "zap.local",
				Address: "zap.local:22",
			},
			{
				Name:    "zap.non_local",
				Address: "zap.non_local:22",
			},
		})
		require.NoError(t, err)
		require.ElementsMatch(t, []*wishlist.Endpoint{
			{
				Name:    "foo.local",
				Address: "foo.local:22",
				User:    "carlos",
			},
			{
				Name:         "zap.local",
				Address:      "zap.local:22",
				User:         "carlos",
				ForwardAgent: true,
			},
			{
				Name:    "zap.non_local",
				Address: "zap.non_local:22",
			},
		}, endpoints)
	})
}

func TestParseReader(t *testing.T) {
	t.Run("error", func(t *testing.T) {
		endpoints, err := ParseReader(
			&fakeNamedReader{
				reader: iotest.ErrReader(fmt.Errorf("any")),
				name:   "",
			},
			nil,
		)
		require.EqualError(t, err, "failed to read config: any")
		require.Empty(t, endpoints)
	})
}

//go:embed testdata/include
var includeFile []byte

//go:embed testdata/1.included
var includedFile1 []byte

//go:embed testdata/2.included
var includedFile2 []byte

func TestParseIncludes(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "config")
	require.NoError(t, os.WriteFile(path, includeFile, 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmp, "1.included"), includedFile1, 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmp, "2.included"), includedFile2, 0644))

	endpoints, err := ParseFile(path, nil)
	require.NoError(t, err)
	require.ElementsMatch(t, []*wishlist.Endpoint{
		{
			Name:          "test.foo.bar",
			Address:       "test.foo.bar:2222",
			User:          "ciclano",
			IdentityFiles: []string{"~/.ssh/id_rsa2", "~/.ssh/other_id"},
		},
		{
			Name:                     "something.else",
			Address:                  "something.else:2323",
			User:                     "ciclano",
			PreferredAuthentications: []string{"password"},
		},
	}, endpoints)
}

func TestParseIncludesAbsPath(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "config")
	includedPath := filepath.Join(tmp, "1.included")
	require.NoError(t, os.WriteFile(path, []byte(fmt.Sprintf(`
		Include %s

		Host test.foo.bar
			User me
	`, includedPath)), 0644))
	require.NoError(t, os.WriteFile(includedPath, includedFile1, 0644))

	endpoints, err := ParseFile(path, nil)
	require.NoError(t, err)
	require.ElementsMatch(t, []*wishlist.Endpoint{
		{
			Name:          "test.foo.bar",
			Address:       "test.foo.bar:22",
			User:          "me",
			IdentityFiles: []string{"~/.ssh/id_rsa2"},
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
	), nil)

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
	require.Equal(t, expectedErr, m.forEach(func(s string, _ hostinfo, e error) error {
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

type fakeNamedReader struct {
	reader io.Reader
	name   string
}

// Name implements NamedReader.
func (r *fakeNamedReader) Name() string {
	return r.name
}

// Read implements NamedReader.
func (r *fakeNamedReader) Read(p []byte) (n int, err error) {
	return r.reader.Read(p)
}

func newNamedReader(s, name string) NamedReader {
	return &fakeNamedReader{
		reader: strings.NewReader(s),
		name:   name,
	}
}
