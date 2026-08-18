// Package multicast contains multicast connections.
package multicast

import (
	"net"
	"syscall"
)

// Conn is a Multicast connection.
type Conn interface {
	net.PacketConn
	SetReadBuffer(int) error
	SyscallConn() (syscall.RawConn, error)
}

type genericIP struct {
	Family int
	IPProto int
	MulticastHops int
}
