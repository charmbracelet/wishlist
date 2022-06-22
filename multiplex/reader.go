package multiplex

import (
	"bytes"
	"io"
	"log"
)

// ResetableReader is an io.Reader that can be "reset", i.e.: cleared of
// everything that was not yet read.
type ResetableReader interface {
	io.Reader
	Reset()
}

// Reader keeps reading r and writing to 2 other readers, which are returned.
// It stops only when the done channel is notified.
func Reader(r io.Reader, done <-chan bool) (ResetableReader, ResetableReader) {
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
						log.Println("ignored multiplex read error:", err)
					}
					continue
				}
				if _, err := w.Write(buf[:n]); err != nil {
					log.Println("multiplex write error:", err)
				}
			}
		}
	}()

	// waits for the first read to start
	<-rch
	return &r1, &r2
}
