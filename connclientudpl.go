package gortsplib

import (
	"net"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/aler9/gortsplib/pkg/base"
	"github.com/aler9/gortsplib/pkg/multibuffer"
)

type connClientUDPListener struct {
	c              *ConnClient
	pc             net.PacketConn
	remoteIp       net.IP
	remoteZone     string
	remotePort     int
	udpFrameBuffer *multibuffer.MultiBuffer
	trackId        int
	streamType     StreamType
	running        bool

	done chan struct{}
}

func newConnClientUDPListener(c *ConnClient, port int) (*connClientUDPListener, error) {
	pc, err := c.d.ListenPacket("udp", ":"+strconv.FormatInt(int64(port), 10))
	if err != nil {
		return nil, err
	}

	return &connClientUDPListener{
		c:              c,
		pc:             pc,
		udpFrameBuffer: multibuffer.New(c.d.ReadBufferCount+1, 2048),
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

		if !l.remoteIp.Equal(uaddr.IP) || l.remotePort != uaddr.Port {
			continue
		}

		atomic.StoreInt64(l.c.udpLastFrameTimes[l.trackId], time.Now().Unix())

		l.c.rtcpReceivers[l.trackId].OnFrame(l.streamType, buf[:n])

		l.c.readFrame <- base.InterleavedFrame{
			TrackId:    l.trackId,
			StreamType: l.streamType,
			Content:    buf[:n],
		}
	}
}

func (l *connClientUDPListener) write(buf []byte) error {
	_, err := l.pc.WriteTo(buf, &net.UDPAddr{
		IP:   l.remoteIp,
		Zone: l.remoteZone,
		Port: l.remotePort,
	})
	return err
}
