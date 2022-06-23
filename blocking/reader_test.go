package blocking

import (
	"io"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBlockingReader(t *testing.T) {
	r := New(&testReader{
		results: []readResult{
			{n: 0, e: io.EOF},              // EOF should be ignored
			{n: 12, e: nil},                // return normally
			{n: 0, e: io.ErrUnexpectedEOF}, // return err
		},
	})

	n, err := r.Read([]byte{})
	require.Equal(t, 12, n)
	require.NoError(t, err)

	n, err = r.Read([]byte{})
	require.Equal(t, 0, n)
	require.Equal(t, err, io.ErrUnexpectedEOF)
}

type testReader struct {
	results []readResult
	last    int32
}

func (r *testReader) Read(data []byte) (int, error) {
	res := r.results[atomic.AddInt32(&r.last, 1)-1]
	return res.n, res.e
}

type readResult struct {
	n int
	e error
}
