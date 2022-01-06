package wishlist

import (
	"bytes"
	"io"
	"log"
)

// multiplex keeps reading r and writing to 2 other readers, which are returned.
// it stops only when done is notified or r EOF's.
func multiplex(r io.Reader, done <-chan bool) (io.Reader, io.Reader) {
	var r1 bytes.Buffer
	var r2 bytes.Buffer

	w := io.MultiWriter(&r1, &r2)
	go func() {
		for {
			select {
			case <-done:
				return
			default:
				buf := [256]byte{}
				n, err := r.Read(buf[:])
				if err != nil {
					if err == io.EOF {
						return
					}
					log.Println("multiplex error:", err)
					continue
				}
				if n == 0 {
					continue
				}
				if _, err := w.Write(buf[:n]); err != nil {
					log.Println("multiplex error:", err)
				}
			}
		}
	}()

	return &r1, &r2
}
