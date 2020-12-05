package gortsplib

import (
	"net"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/aler9/gortsplib/pkg/multibuffer"
)

const (
	// use the same buffer size as gstreamer's rtspsrc
	connClientUDPKernelReadBufferSize = 0x80000

	connClientUDPReadBufferSize = 2048
)

type connClientUDPListener struct {
	c              *ConnClient
	pc             net.PacketConn
	remoteIP       net.IP
	remoteZone     string
	remotePort     int
	udpFrameBuffer *multibuffer.MultiBuffer
	trackID        int
	streamType     StreamType
	running        bool

	done chan struct{}
}

func newConnClientUDPListener(c *ConnClient, port int) (*connClientUDPListener, error) {
	pc, err := c.d.ListenPacket("udp", ":"+strconv.FormatInt(int64(port), 10))
	if err != nil {
		return nil, err
	}

	err = pc.(*net.UDPConn).SetReadBuffer(connClientUDPKernelReadBufferSize)
	if err != nil {
		return nil, err
	}

	return &connClientUDPListener{
		c:              c,
		pc:             pc,
		udpFrameBuffer: multibuffer.New(c.d.ReadBufferCount, connClientUDPReadBufferSize),
	}, nil
}

func (l *connClientUDPListener) close() {
	if l.running {
		l.stop()
	}
	l.pc.Close()
}

func (l *connClientUDPListener) start() {
	l.running = true
	l.pc.SetReadDeadline(time.Time{})
	l.done = make(chan struct{})
	go l.run()
}

func (l *connClientUDPListener) stop() {
	l.pc.SetReadDeadline(time.Now())
	<-l.done
}

func (l *connClientUDPListener) run() {
	defer close(l.done)

	for {
		buf := l.udpFrameBuffer.Next()
		n, addr, err := l.pc.ReadFrom(buf)
		if err != nil {
			return
		}

		uaddr := addr.(*net.UDPAddr)

		if !l.remoteIP.Equal(uaddr.IP) || l.remotePort != uaddr.Port {
			continue
		}

		now := time.Now()
		atomic.StoreInt64(l.c.udpLastFrameTimes[l.trackID], now.Unix())
		l.c.rtcpReceivers[l.trackID].ProcessFrame(now, l.streamType, buf[:n])

		l.c.readCB(l.trackID, l.streamType, buf[:n])
	}
}

func (l *connClientUDPListener) write(buf []byte) error {
	l.pc.SetWriteDeadline(time.Now().Add(l.c.d.WriteTimeout))
	_, err := l.pc.WriteTo(buf, &net.UDPAddr{
		IP:   l.remoteIP,
		Zone: l.remoteZone,
		Port: l.remotePort,
	})
	return err
}
