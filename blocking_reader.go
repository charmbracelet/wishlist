package wishlist

import (
	"io"
	"time"
)

type blockingReader struct {
	r io.Reader
}

type readResult struct {
	n int
	e error
}

func (r blockingReader) Read(data []byte) (int, error) {
	readch := make(chan readResult, 1)

	go func() {
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
