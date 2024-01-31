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
		"desc\nhttps://example.com\nssh://foo.bar:22",
		s.Description(),
	)
}

func TestWithSSHURL(t *testing.T) {
	require.Equal(
		t,
		"ssh://localhost:22",
		withSSHURL(&Endpoint{
			Address: "localhost:22",
		}, styles{}),
	)
}

func TestWithDescription(t *testing.T) {
	t.Run("no description", func(t *testing.T) {
		require.Equal(
			t,
			"no description",
			withDescription(&Endpoint{}, styles{}),
		)
	})
	t.Run("multiline", func(t *testing.T) {
		require.Equal(
			t,
			"foo",
			withDescription(&Endpoint{
				Desc: "foo\n\nbar\n\nsfsdfsd\n",
			}, styles{}),
		)
	})
	t.Run("simple", func(t *testing.T) {
		require.Equal(
			t,
			"foobar desc",
			withDescription(&Endpoint{
				Desc: "foobar desc",
			}, styles{}),
		)
	})
}

func TestWithLink(t *testing.T) {
	t.Run("no link", func(t *testing.T) {
		require.Equal(
			t,
			"no link",
			withLink(&Endpoint{}, styles{}),
		)
	})
	t.Run("url only", func(t *testing.T) {
		require.Equal(
			t,
			"https://example.com",
			withLink(&Endpoint{
				Link: Link{
					URL: "https://example.com",
				},
			}, styles{}),
		)
	})
	t.Run("url and name", func(t *testing.T) {
		require.Equal(
			t,
			"example https://example.com",
			withLink(&Endpoint{
				Link: Link{
					Name: "example",
					URL:  "https://example.com",
				},
			}, styles{}),
		)
	})
}
