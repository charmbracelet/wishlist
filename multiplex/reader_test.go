package multiplex

import (
	"bytes"
	"io"
	"testing"
	"testing/iotest"

	"github.com/stretchr/testify/require"
)

func TestMultiplex(t *testing.T) {
	t.Run("clean", func(t *testing.T) {
		var b bytes.Buffer

		const s = "this is not a test, this is not a test"
		_, err := b.WriteString(s)
		require.NoError(t, err)

		done := make(chan bool, 1)
		t.Cleanup(func() { done <- true })
		r1, r2 := Reader(&b, done)

		b1, err := io.ReadAll(r1)
		require.NoError(t, err)
		require.Equal(t, s, string(b1))

		b2, err := io.ReadAll(r2)
		require.NoError(t, err)
		require.Equal(t, s, string(b2))
	})

	t.Run("reset", func(t *testing.T) {
		var b bytes.Buffer

		const s = "this in the other hand is a test"
		_, err := b.WriteString(s)
		require.NoError(t, err)

		done := make(chan bool, 1)
		t.Cleanup(func() { done <- true })
		r1, r2 := Reader(&b, done)

		b1, err := io.ReadAll(r1)
		require.NoError(t, err)
		require.Equal(t, s, string(b1))

		r2.Reset()
		b2, err := io.ReadAll(r2)
		require.NoError(t, err)
		require.Empty(t, string(b2))
	})

	t.Run("read err", func(t *testing.T) {
		r := iotest.ErrReader(io.ErrClosedPipe)
		done := make(chan bool, 1)
		t.Cleanup(func() { done <- true })
		r1, r2 := Reader(r, done)

		b1, err := io.ReadAll(r1)
		require.NoError(t, err)
		require.Empty(t, string(b1))

		b2, err := io.ReadAll(r2)
		require.NoError(t, err)
		require.Empty(t, string(b2))
	})
}
