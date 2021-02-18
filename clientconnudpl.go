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
	clientConnUDPKernelReadBufferSize = 0x80000
)

type clientConnUDPListener struct {
	c              *ClientConn
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

func newClientConnUDPListener(c *ClientConn, port int) (*clientConnUDPListener, error) {
	pc, err := c.conf.ListenPacket("udp", ":"+strconv.FormatInt(int64(port), 10))
	if err != nil {
		return nil, err
	}

	err = pc.(*net.UDPConn).SetReadBuffer(clientConnUDPKernelReadBufferSize)
	if err != nil {
		return nil, err
	}

	return &clientConnUDPListener{
		c:              c,
		pc:             pc,
		udpFrameBuffer: multibuffer.New(uint64(c.conf.ReadBufferCount), uint64(c.conf.ReadBufferSize)),
	}, nil
}

func (l *clientConnUDPListener) close() {
	if l.running {
		l.stop()
	}
	l.pc.Close()
}

func (l *clientConnUDPListener) start() {
	l.running = true
	l.pc.SetReadDeadline(time.Time{})
	l.done = make(chan struct{})
	go l.run()
}

func (l *clientConnUDPListener) stop() {
	l.pc.SetReadDeadline(time.Now())
	<-l.done
}

func (l *clientConnUDPListener) run() {
	defer close(l.done)

	for {
		buf := l.udpFrameBuffer.Next()
		n, addr, err := l.pc.ReadFrom(buf)
		if err != nil {
			return
		}

		uaddr := addr.(*net.UDPAddr)

		if !l.remoteIP.Equal(uaddr.IP) || (l.remotePort != 0 && l.remotePort != uaddr.Port) {
			continue
		}

		now := time.Now()
		atomic.StoreInt64(l.c.udpLastFrameTimes[l.trackID], now.Unix())
		l.c.rtcpReceivers[l.trackID].ProcessFrame(now, l.streamType, buf[:n])

		l.c.readCB(l.trackID, l.streamType, buf[:n])
	}
}

func (l *clientConnUDPListener) write(buf []byte) error {
	l.pc.SetWriteDeadline(time.Now().Add(l.c.conf.WriteTimeout))
	_, err := l.pc.WriteTo(buf, &net.UDPAddr{
		IP:   l.remoteIP,
		Zone: l.remoteZone,
		Port: l.remotePort,
	})
	return err
}
