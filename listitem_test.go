package wishlist

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestItemWrapper(t *testing.T) {
	s := ItemWrapper{
		endpoint: &Endpoint{
			Name: "name",
			Desc: "desc",
			Link: Link{
				URL: "https://example.com",
			},
			Address: "foo.bar:22",
		},
		descriptors: []descriptor{
			withDescription,
			withLink,
			withSSHURL,
		},
	}

	require.Equal(t, "name", s.Title())
	require.Equal(t, "name", s.FilterValue())
	require.Equal(
		t,
		"desc\n\x1b]8;;https://example.com\x1b\\https://example.com\x1b]8;;\x1b\\\n\x1b]8;;ssh://foo.bar:22\x1b\\ssh://foo.bar:22\x1b]8;;\x1b\\",
		s.Description(),
	)
}

func TestWithSSHURL(t *testing.T) {
	require.Equal(
		t,
		"\x1b]8;;ssh://localhost:22\x1b\\ssh://localhost:22\x1b]8;;\x1b\\",
		withSSHURL(&Endpoint{
			Address: "localhost:22",
		}),
	)
}

func TestWithDescription(t *testing.T) {
	t.Run("no description", func(t *testing.T) {
		require.Equal(
			t,
			"\x1b[3;2mno description\x1b[0m",
			withDescription(&Endpoint{}),
		)
	})
	t.Run("multiline", func(t *testing.T) {
		require.Equal(
			t,
			"foo",
			withDescription(&Endpoint{
				Desc: "foo\n\nbar\n\nsfsdfsd\n",
			}),
		)
	})
	t.Run("simple", func(t *testing.T) {
		require.Equal(
			t,
			"foobar desc",
			withDescription(&Endpoint{
				Desc: "foobar desc",
			}),
		)
	})
}

func TestWithLink(t *testing.T) {
	t.Run("no link", func(t *testing.T) {
		require.Equal(
			t,
			"\x1b[3;2mno link\x1b[0m",
			withLink(&Endpoint{}),
		)
	})
	t.Run("url only", func(t *testing.T) {
		require.Equal(
			t,
			"\x1b]8;;https://example.com\x1b\\https://example.com\x1b]8;;\x1b\\",
			withLink(&Endpoint{
				Link: Link{
					URL: "https://example.com",
				},
			}),
		)
	})
	t.Run("url and name", func(t *testing.T) {
		require.Equal(
			t,
			"\x1b]8;;https://example.com\x1b\\example\x1b]8;;\x1b\\",
			withLink(&Endpoint{
				Link: Link{
					Name: "example",
					URL:  "https://example.com",
				},
			}),
		)
	})
}
