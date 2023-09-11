//go:build !windows
// +build !windows

package multicast

import (
	"net"

	"golang.org/x/net/ipv4"
)

func setupReadFrom(c *ipv4.PacketConn) error {
	return c.SetControlMessage(ipv4.FlagDst, true)
}

func readFrom(c *ipv4.PacketConn, destIP net.IP, b []byte) (int, net.Addr, error) {
	for {
		n, cm, src, err := c.ReadFrom(b)
		if err != nil {
			return 0, nil, err
		}

		// a multicast connection can receive packets
		// addressed to groups joined by other connections.
		// discard them.
		if !cm.Dst.Equal(destIP) {
			continue
		}

		return n, src, nil
	}
}
