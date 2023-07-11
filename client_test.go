package wishlist

import (
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestClosers(t *testing.T) {
	var b1, b2, b3, b4 atomic.Bool
	cl := closers{
		func() error {
			defer func() { b1.Store(true) }()
			t.Log("close 1")
			return nil
		},
		func() error {
			defer func() { b2.Store(true) }()
			t.Log("close 2")
			return fmt.Errorf("fake error")
		},
		func() error {
			defer func() { b3.Store(true) }()
			t.Log("close 3")
			return nil
		},
		func() error {
			defer func() { b4.Store(true) }()
			t.Log("close 4")
			return fmt.Errorf("another error")
		},
	}
	cl.close()

	for _, b := range []*atomic.Bool{&b1, &b2, &b3, &b4} {
		require.True(t, b.Load())
	}
}
