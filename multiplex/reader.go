package multiplex

import (
	"bytes"
	"io"
	"log"
	"sync"
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
	var r1 syncWriter
	var r2 syncWriter

	rch := make(chan bool, 1)
	var once sync.Once

	w := io.MultiWriter(&r1, &r2)
	go func() {
		for {
			select {
			case <-done:
				return
			default:
				once.Do(func() {
					rch <- true
				})

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

var (
	_ io.ReadWriter   = &syncWriter{}
	_ ResetableReader = &syncWriter{}
)

type syncWriter struct {
	b  bytes.Buffer
	mu sync.RWMutex
}

// Reset implements ResetableReader.
func (w *syncWriter) Reset() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.b.Reset()
}

// Read implements io.Reader.
func (w *syncWriter) Read(p []byte) (n int, err error) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.b.Read(p) //nolint: wrapcheck
}

// Write implements io.Writer.
func (w *syncWriter) Write(p []byte) (n int, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.b.Write(p) //nolint: wrapcheck
}
