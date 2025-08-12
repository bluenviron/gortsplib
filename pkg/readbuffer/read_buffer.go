// Package readbuffer contains a function to get the read buffer size of a socket.
package readbuffer

import (
	"net"
	"syscall"
)

// PacketConn is a packet connection.
type PacketConn interface {
	net.PacketConn
	SyscallConn() (syscall.RawConn, error)
}
