package wishlist

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSplitJump(t *testing.T) {
	for jump, expected := range map[string]struct {
		user, addr string
	}{
		"foo":                   {addr: "foo:22"},
		"foo:22":                {addr: "foo:22"},
		"foo:2234":              {addr: "foo:2234"},
		"user@foo":              {user: "user", addr: "foo:22"},
		"user@foo:22":           {user: "user", addr: "foo:22"},
		"user@foo:2223":         {user: "user", addr: "foo:2223"},
		"user@bar@foo:22":       {user: "user@bar", addr: "foo:22"},
		"user@bar@zaz@foo:22":   {user: "user@bar@zaz", addr: "foo:22"},
		"user@bar@zaz@foo":      {user: "user@bar@zaz", addr: "foo:22"},
		"user@bar@zaz@foo:2323": {user: "user@bar@zaz", addr: "foo:2323"},
	} {
		t.Run(jump, func(t *testing.T) {
			user, addr := splitJump(jump)
			require.Equal(t, expected.user, user)
			require.Equal(t, expected.addr, addr)
		})
	}
}
