package multiplex

import (
	"bytes"
	"io"
	"testing"
	"testing/iotest"
	"time"

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

		for _, r := range []io.Reader{r1, r2} {
			require.Eventually(t, func() bool {
				bts, err := io.ReadAll(r)
				return err == nil && s == string(bts)
			}, time.Second*2, 100*time.Millisecond)
		}
	})

	t.Run("reset", func(t *testing.T) {
		var b bytes.Buffer

		const s = "this in the other hand is a test"
		_, err := b.WriteString(s)
		require.NoError(t, err)

		done := make(chan bool, 1)
		t.Cleanup(func() { done <- true })
		r1, r2 := Reader(&b, done)

		for _, r := range []io.Reader{r1, r2} {
			require.Eventually(t, func() bool {
				bts, err := io.ReadAll(r)
				return err == nil && s == string(bts)
			}, time.Second*2, 100*time.Millisecond)
		}

		for _, r := range []ResetableReader{r1, r2} {
			r.Reset()
			bts, err := io.ReadAll(r)
			require.NoError(t, err)
			require.Empty(t, bts)
		}
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

func FuzzMultiplex(f *testing.F) {
	for _, seed := range [][]byte{{}, {0}, {9}, {0xa}, {0xf}, {1, 2, 3, 4}, nil, []byte("some string\n")} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, in []byte) {
		var b bytes.Buffer
		_, err := b.Write(in)
		require.NoError(t, err)

		done := make(chan bool, 1)
		t.Cleanup(func() { done <- true })
		r1, r2 := Reader(&b, done)

		for _, r := range []io.Reader{r1, r2} {
			require.Eventually(t, func() bool {
				bts, err := io.ReadAll(r)
				return err == nil && bytes.Equal(bts, in)
			}, time.Second*2, 100*time.Millisecond)
		}

		for _, r := range []ResetableReader{r1, r2} {
			r.Reset()
			bts, err := io.ReadAll(r)
			require.NoError(t, err)
			require.Empty(t, bts)
		}
	})
}
