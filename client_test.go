package wishlist

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestClosers(t *testing.T) {
	cl := closers{
		func() error {
			t.Log("close 1")
			return nil
		},
		func() error {
			t.Log("close 2")
			return fmt.Errorf("fake error")
		},
	}
	cl.close()
}

func TestFirstNonEmpty(t *testing.T) {
	require.Equal(t, "a", firstNonEmpty("", "a"))
	require.Equal(t, "", firstNonEmpty("", ""))
}
