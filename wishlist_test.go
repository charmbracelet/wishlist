package wishlist

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEndointToListItems(t *testing.T) {
	result := endpointsToListItems([]*Endpoint{
		{
			Name:    "name",
			Address: "anything",
		},
	})

	require.Len(t, result, 1)
	item := result[0]
	require.Equal(t, "name", item.FilterValue())
}

func TestNewWishlist(t *testing.T) {
	t.Run("local", func(t *testing.T) {
		lm := NewListing([]*Endpoint{
			{
				Name:    "name",
				Address: "anything",
			},
		}, nil, nil)
		require.Len(t, lm.endpoints, 1)
		require.Nil(t, lm.session)
	})
}
