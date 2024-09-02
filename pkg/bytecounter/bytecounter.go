// Package bytecounter contains a io.ReadWriter wrapper that allows to count read and written bytes, and the number of errors.
package bytecounter

import (
	"io"
	"sync/atomic"
)

// ByteCounter is a io.ReadWriter wrapper that allows to count read and written bytes and errors.
type ByteCounter struct {
	rw       io.ReadWriter
	received *uint64
	sent     *uint64

	readErrors  *uint64
	writeErrors *uint64
}

// New allocates a ByteCounter.
func New(rw io.ReadWriter, received *uint64, sent *uint64, readErrors *uint64, writeErrors *uint64) *ByteCounter {
	if received == nil {
		received = new(uint64)
	}
	if sent == nil {
		sent = new(uint64)
	}
	if readErrors == nil {
		readErrors = new(uint64)
	}
	if writeErrors == nil {
		writeErrors = new(uint64)
	}

	return &ByteCounter{
		rw:          rw,
		received:    received,
		sent:        sent,
		readErrors:  readErrors,
		writeErrors: writeErrors,
	}
}

// Read implements io.ReadWriter.
func (bc *ByteCounter) Read(p []byte) (int, error) {
	n, err := bc.rw.Read(p)
	if err == nil {
		atomic.AddUint64(bc.received, uint64(n))
	} else {
		atomic.AddUint64(bc.readErrors, 1)
	}

	return n, err
}

// Write implements io.ReadWriter.
func (bc *ByteCounter) Write(p []byte) (int, error) {
	n, err := bc.rw.Write(p)
	if err == nil {
		atomic.AddUint64(bc.sent, uint64(n))
	} else {
		atomic.AddUint64(bc.writeErrors, 1)
	}
	return n, err
}

// BytesReceived returns the number of bytes received.
func (bc *ByteCounter) BytesReceived() uint64 {
	return atomic.LoadUint64(bc.received)
}

// BytesSent returns the number of bytes sent.
func (bc *ByteCounter) BytesSent() uint64 {
	return atomic.LoadUint64(bc.sent)
}

// ReadErrors returns the number of read errors.
func (bc *ByteCounter) ReadErrors() uint64 {
	return atomic.LoadUint64(bc.readErrors)
}

// WriteErrors returns the number of write errors.
func (bc *ByteCounter) WriteErrors() uint64 {
	return atomic.LoadUint64(bc.writeErrors)
}
