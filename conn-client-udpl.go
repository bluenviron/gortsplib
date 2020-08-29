package gortsplib

import (
	"net"
	"strconv"
)

// connClientUdpListener is a UDP listener created by SetupUDP() to receive UDP frames.
type connClientUdpListener struct {
	c             *ConnClient
	pc            net.PacketConn
	trackId       int
	streamType    StreamType
	publisherIp   net.IP
	publisherPort int
}

func newConnClientUdpListener(c *ConnClient, port int, trackId int, streamType StreamType) (*connClientUdpListener, error) {
	pc, err := c.conf.ListenPacket("udp", ":"+strconv.FormatInt(int64(port), 10))
	if err != nil {
		return nil, err
	}

	return &connClientUdpListener{
		c:          c,
		pc:         pc,
		trackId:    trackId,
		streamType: streamType,
	}, nil
}

func (l *connClientUdpListener) close() {
	l.pc.Close()
}

// Read reads a frame from the publisher.
func (l *connClientUdpListener) Read(buf []byte) (int, error) {
	for {
		n, addr, err := l.pc.ReadFrom(buf)
		if err != nil {
			return 0, err
		}

		uaddr := addr.(*net.UDPAddr)

		if !l.publisherIp.Equal(uaddr.IP) || l.publisherPort != uaddr.Port {
			continue
		}

		l.c.rtcpReceivers[l.trackId].OnFrame(l.streamType, buf[:n])
		return n, nil
	}
}
