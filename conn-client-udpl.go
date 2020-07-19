package gortsplib

import (
	"net"
	"strconv"
)

// ConnClientUdpListener is a UDP listener created by SetupUDP() to receive UDP frames.
type ConnClientUdpListener struct {
	c             *ConnClient
	pc            net.PacketConn
	trackId       int
	streamType    StreamType
	publisherIp   net.IP
	publisherPort int
}

func newConnClientUdpListener(c *ConnClient, port int, trackId int, streamType StreamType) (*ConnClientUdpListener, error) {
	pc, err := net.ListenPacket("udp", ":"+strconv.FormatInt(int64(port), 10))
	if err != nil {
		return nil, err
	}

	return &ConnClientUdpListener{
		c:          c,
		pc:         pc,
		trackId:    trackId,
		streamType: streamType,
	}, nil
}

// Close closes the listener.
func (l *ConnClientUdpListener) Close() {
	l.pc.Close()
}

// Read reads a frame from the publisher.
func (l *ConnClientUdpListener) Read(buf []byte) (int, error) {
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
