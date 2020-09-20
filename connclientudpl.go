package gortsplib

import (
	"net"
	"strconv"
	"sync/atomic"
	"time"
)

// UDPReadFunc is a function used to read UDP packets.
type UDPReadFunc func() ([]byte, error)

type connClientUDPListener struct {
	c             *ConnClient
	pc            net.PacketConn
	trackId       int
	streamType    StreamType
	publisherIp   net.IP
	publisherPort int
	udpFrameReadBuf *MultiBuffer
}

func newConnClientUDPListener(c *ConnClient, port int, trackId int, streamType StreamType) (*connClientUDPListener, error) {
	pc, err := c.conf.ListenPacket("udp", ":"+strconv.FormatInt(int64(port), 10))
	if err != nil {
		return nil, err
	}

	return &connClientUDPListener{
		c:          c,
		pc:         pc,
		trackId:    trackId,
		streamType: streamType,
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

		atomic.StoreInt64(l.c.udpLastFrameTimes[l.trackId], time.Now().Unix())

		l.c.rtcpReceivers[l.trackId].OnFrame(l.streamType, buf[:n])

		return buf[:n], nil
	}
}
