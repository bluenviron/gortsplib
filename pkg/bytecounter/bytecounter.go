// Package bytecounter contains a io.ReadWriter wrapper that allows to count read and written bytes.
package bytecounter

import (
	"io"
	"sync/atomic"
)

// ByteCounter is a io.ReadWriter wrapper that allows to count read and written bytes.
type ByteCounter struct {
	rw      io.ReadWriter
	read    uint64
	written uint64
}

// New allocates a ByteCounter.
func New(rw io.ReadWriter) *ByteCounter {
	return &ByteCounter{
		rw: rw,
	}
}

// Read implements io.ReadWriter.
func (bc *ByteCounter) Read(p []byte) (int, error) {
	n, err := bc.rw.Read(p)
	atomic.AddUint64(&bc.read, uint64(n))
	return n, err
}

// Write implements io.ReadWriter.
func (bc *ByteCounter) Write(p []byte) (int, error) {
	n, err := bc.rw.Write(p)
	atomic.AddUint64(&bc.written, uint64(n))
	return n, err
}

// BytesReceived returns the number of read bytes.
func (bc *ByteCounter) BytesReceived() uint64 {
	return atomic.LoadUint64(&bc.read)
}

// BytesSent returns the number of written bytes.
func (bc *ByteCounter) BytesSent() uint64 {
	return atomic.LoadUint64(&bc.written)
}
