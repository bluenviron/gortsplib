// Package readbuffer contains a function to set the read buffer size of a UDP socket.
package readbuffer

import (
	"syscall"
)

// PacketConn is a packet connection.
type PacketConn interface {
	SyscallConn() (syscall.RawConn, error)
}

// SetReadBuffer sets the read buffer size of the UDP connection and checks that it was set correctly.
func SetReadBuffer(pc PacketConn, size int) error {
	rawConn, err := pc.SyscallConn()
	if err != nil {
		panic(err)
	}

	return SetReadBufferRaw(rawConn, size)
}
