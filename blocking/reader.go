package blocking

import (
	"io"
	"time"
)

// Reader is an io.Reader that blocks until the underlying reader until
// returns something other than io.EOF.
//
// On EOF, it'll keep trying to read again every 10ms.
//
// The purpose of this is to be used to "emulate a STDIN" (which never EOFs)
// from another io.Reader, for example, a bytes.Buffer.
type Reader struct {
	r io.Reader
}

// New wraps a given io.Reader into a BlockingReader.
func New(r io.Reader) Reader {
	return Reader{r: r}
}

func (r Reader) Read(data []byte) (int, error) {
	for {
		n, err := r.r.Read(data)
		if err == nil || err != io.EOF {
			// nolint:wrapcheck
			return n, err
		}
		// 10ms is not that much a magic number, more like a guess.
		// nolint:gomnd
		time.Sleep(10 * time.Millisecond)
	}
}
