package wishlist

import (
	"bytes"
	"io"
	"log"
)

// multiplex keeps reading r and writing to 2 other readers, which are returned.
// it stops only when the done channel is notified.
func multiplex(r io.Reader, done <-chan bool) (io.Reader, io.Reader) {
	var r1 bytes.Buffer
	var r2 bytes.Buffer

	rch := make(chan bool, 1)

	w := io.MultiWriter(&r1, &r2)
	go func() {
		first := true
		for {
			select {
			case <-done:
				return
			default:
				if first {
					first = false
					rch <- true
				}
				buf := [256]byte{}
				n, err := r.Read(buf[:])
				if err != nil {
					if err != io.EOF {
						log.Println("multiplex error:", err)
					}
					continue
				}
				if _, err := w.Write(buf[:n]); err != nil {
					log.Println("multiplex error:", err)
				}
			}
		}
	}()

	// waits for the first read to start
	<-rch
	return &r1, &r2
}
