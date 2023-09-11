// Package multicast contains multicast connections.
package multicast

import (
	"net"
)

// Conn is a Multicast connection.
type Conn interface {
	net.PacketConn
	SetReadBuffer(int) error
}
