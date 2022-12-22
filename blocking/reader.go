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
// The purpose of this is to be used to "emulate a STDIN"
// from another io.Reader, for example, a bytes.Buffer.
//
// We need that because when we connect into an app through wishlist, we need
// to keep a copy of STDIN (named handoffstdin in most places). That copy is a
// bytes.Buffer, which would EOF on last byte, but we are still writing to
// it... so it shouldn't really EOF. Hence, this Reader. It'll never EOF, and
// will keep trying to read until another error happens.
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
			//nolint:wrapcheck
			return n, err
		}
		// 10ms is not that much a magic number, more like a guess.
		//nolint:gomnd
		time.Sleep(10 * time.Millisecond)
	}
}
