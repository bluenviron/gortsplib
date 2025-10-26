// Package readbuffer contains a function to set the read buffer size of a UDP socket.
package readbuffer

import (
	"fmt"
	"net"
	"syscall"
)

// PacketConn is a packet connection.
type PacketConn interface {
	net.PacketConn
	SyscallConn() (syscall.RawConn, error)
	SetReadBuffer(bytes int) error
}

// SetReadBuffer sets the read buffer size of the UDP connection and checks that it was set correctly.
func SetReadBuffer(pc PacketConn, size int) error {
	err := pc.SetReadBuffer(size)
	if err != nil {
		return err
	}

	v, err := ReadBuffer(pc)
	if err != nil {
		return err
	}

	if v != size {
		return fmt.Errorf("unable to set read buffer size to %v, check that the operating system allows that",
			size)
	}

	return nil
}
