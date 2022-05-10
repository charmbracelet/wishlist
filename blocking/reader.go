package blocking

import (
	"io"
	"time"
)

// Reader is an io.Reader that blocks until the underlying reader until
// returns something other than io.EOF.
//
// on EOF, it'll keep trying to read every 10ms.
//
// The purpose of this is to be used to "emulate a STDIN" (which never EOFs)
// from another io.Reader, e.g. a bytes.Buffer.
type Reader struct {
	r io.Reader
}

// New wraps a given io.Reader into a BlockingReader.
func New(r io.Reader) Reader {
	return Reader{r: r}
}

type readResult struct {
	n int
	e error
}

func (r Reader) Read(data []byte) (int, error) {
	readch := make(chan readResult, 1)

	go func() {
		// 10ms is not that much a magic number, more like a guess.
		// nolint:gomnd
		ticker := time.NewTicker(time.Millisecond * 10)
		defer ticker.Stop()

		for range ticker.C {
			n, err := r.r.Read(data)
			if err != nil && err != io.EOF {
				readch <- readResult{n, err}
				return
			}
			if n > 0 {
				readch <- readResult{n, err}
				return
			}
		}
	}()

	res := <-readch
	return res.n, res.e
}
