package wishlist

import (
	"fmt"
	"testing"
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
