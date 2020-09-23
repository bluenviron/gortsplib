package gortsplib

import (
	"net"
	"strconv"
)

type connClientUDPListener struct {
	pc              net.PacketConn
	publisherIp     net.IP
	publisherPort   int
	udpFrameReadBuf *MultiBuffer
}

func newConnClientUDPListener(c *ConnClient, port int) (*connClientUDPListener, error) {
	pc, err := c.conf.ListenPacket("udp", ":"+strconv.FormatInt(int64(port), 10))
	if err != nil {
		return nil, err
	}

	return &connClientUDPListener{
		pc:              pc,
		udpFrameReadBuf: NewMultiBuffer(c.conf.ReadBufferCount, 2048),
	}, nil
}

func (l *connClientUDPListener) close() {
	l.pc.Close()
}

func (l *connClientUDPListener) read() ([]byte, error) {
	for {
		buf := l.udpFrameReadBuf.Next()
		n, addr, err := l.pc.ReadFrom(buf)
		if err != nil {
			return nil, err
		}

		uaddr := addr.(*net.UDPAddr)

		if !l.publisherIp.Equal(uaddr.IP) || l.publisherPort != uaddr.Port {
			continue
		}

		return buf[:n], nil
	}
}
