//go:build windows
// +build windows

package multicast

import (
	"net"

	"golang.org/x/net/ipv4"
)

func setupReadFrom(c *ipv4.PacketConn) error {
	return nil
}

func readFrom(c *ipv4.PacketConn, destIP net.IP, b []byte) (int, net.Addr, error) {
	n, _, src, err := c.ReadFrom(b)
	return n, src, err
}
