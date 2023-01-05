package srv

import (
	"net"
	"testing"

	"github.com/charmbracelet/wishlist"
	"github.com/stretchr/testify/require"
)

func TestFromRecords(t *testing.T) {
	t.Run("no txt records", func(t *testing.T) {
		require.ElementsMatch(t, []*wishlist.Endpoint{
			{
				Name:    "foo.local",
				Address: "foo.local:2222",
			},
			{
				Name:    "foo.bar",
				Address: "foo.bar:22",
			},
		}, fromRecords([]*net.SRV{
			{
				Target:   "foo.bar",
				Port:     22,
				Priority: 10,
				Weight:   10,
			},
			{
				Target:   "foo.local",
				Port:     2222,
				Priority: 10,
				Weight:   10,
			},
		}, nil))
	})

	t.Run("with txt records", func(t *testing.T) {
		require.ElementsMatch(t, []*wishlist.Endpoint{
			{
				Name:    "local-foo",
				Address: "foo.local:2222",
			},
			{
				Name:    "foobar",
				Address: "foo.bar:22",
			},
		}, fromRecords([]*net.SRV{
			{
				Target:   "foo.bar",
				Port:     22,
				Priority: 10,
				Weight:   10,
			},
			{
				Target:   "foo.local",
				Port:     2222,
				Priority: 10,
				Weight:   10,
			},
		}, []string{
			"wishlist.name foo.bar:22=foobar",
			"wishlist.name foo.local:2222=local-foo",
		}))
	})
}
