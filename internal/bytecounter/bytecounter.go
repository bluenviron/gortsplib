// Package bytecounter contains a io.ReadWriter wrapper that allows to count read and written bytes.
package bytecounter

import (
	"io"
	"sync/atomic"
)

// ByteCounter is a io.ReadWriter wrapper that allows to count read and written bytes.
type ByteCounter struct {
	rw       io.ReadWriter
	received *atomic.Uint64
	sent     *atomic.Uint64
}

// New allocates a ByteCounter.
func New(rw io.ReadWriter, received *atomic.Uint64, sent *atomic.Uint64) *ByteCounter {
	if received == nil {
		received = new(atomic.Uint64)
	}
	if sent == nil {
		sent = new(atomic.Uint64)
	}

	return &ByteCounter{
		rw:       rw,
		received: received,
		sent:     sent,
	}
}

// Read implements io.ReadWriter.
func (bc *ByteCounter) Read(p []byte) (int, error) {
	n, err := bc.rw.Read(p)
	bc.received.Add(uint64(n))
	return n, err
}

// Write implements io.ReadWriter.
func (bc *ByteCounter) Write(p []byte) (int, error) {
	n, err := bc.rw.Write(p)
	bc.sent.Add(uint64(n))
	return n, err
}

// BytesReceived returns the number of bytes received.
func (bc *ByteCounter) BytesReceived() uint64 {
	return bc.received.Load()
}

// BytesSent returns the number of bytes sent.
func (bc *ByteCounter) BytesSent() uint64 {
	return bc.sent.Load()
}
