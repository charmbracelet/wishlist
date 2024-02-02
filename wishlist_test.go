package wishlist

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEndointToListItems(t *testing.T) {
	result := endpointsToListItems([]*Endpoint{
		{
			Name:    "name",
			Address: "anything",
		},
		{
			// invalid
		},
	}, nil, makeStyles(testRenderer))

	require.Len(t, result, 1)
	item := result[0]
	require.Equal(t, "name", item.FilterValue())
}

func TestNewWishlist(t *testing.T) {
	t.Run("local", func(t *testing.T) {
		cl := NewLocalSSHClient()
		lm := NewLocalListing([]*Endpoint{
			{
				Name:    "name",
				Address: "anything",
			},
		}, cl, testRenderer)
		require.Len(t, lm.endpoints, 1)
		require.Equal(t, lm.client, cl)
	})
}

func TestFeatures(t *testing.T) {
	t.Run("complete", func(t *testing.T) {
		descriptors := features([]*Endpoint{
			{
				// invalid
				Desc: "desc",
			},
			{
				Name:    "foo",
				Address: "foo:22",
			},
			{
				Name:    "foo",
				Address: "foo:22",
				Desc:    "desc",
			},
			{
				Name:    "foo",
				Address: "foo:22",
				Link: Link{
					URL: "link",
				},
			},
		})

		require.Len(t, descriptors, 3)
	})

	t.Run("simple", func(t *testing.T) {
		descriptors := features([]*Endpoint{
			{
				// invalid
				Desc: "desc",
			},
			{
				Name:    "foo",
				Address: "foo:22",
			},
		})
		require.Len(t, descriptors, 1)
	})

	t.Run("with desc", func(t *testing.T) {
		descriptors := features([]*Endpoint{
			{
				Name:    "foo",
				Address: "foo:22",
				Desc:    "desc",
			},
		})
		require.Len(t, descriptors, 2)
	})

	t.Run("with link", func(t *testing.T) {
		descriptors := features([]*Endpoint{
			{
				Name:    "foo",
				Address: "foo:22",
				Link: Link{
					URL: "url",
				},
			},
		})
		require.Len(t, descriptors, 2)
	})
}

func TestRootCause(t *testing.T) {
	require.Equal(
		t,
		rootCause(
			fmt.Errorf(
				"foo bar: %w",
				fmt.Errorf(
					"foo bar 2: %w",
					fmt.Errorf(
						"foo bar 3: %w",
						fmt.Errorf("the root cause"),
					),
				),
			),
		).Error(),
		"the root cause",
	)
}

func TestFirstNonEmpty(t *testing.T) {
	require.Equal(t, "a", FirstNonEmpty("", "a"))
	require.Equal(t, "", FirstNonEmpty("", ""))
}
