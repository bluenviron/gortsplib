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
	cc            *ClientConn
	pc            net.PacketConn
	remoteIP      net.IP
	remoteZone    string
	remotePort    int
	trackID       int
	streamType    StreamType
	running       bool
	frameBuffer   *multibuffer.MultiBuffer
	lastFrameTime *int64

	done chan struct{}
}

func newClientConnUDPListener(cc *ClientConn, port int) (*clientConnUDPListener, error) {
	pc, err := cc.conf.ListenPacket("udp", ":"+strconv.FormatInt(int64(port), 10))
	if err != nil {
		return nil, err
	}

	err = pc.(*net.UDPConn).SetReadBuffer(clientConnUDPKernelReadBufferSize)
	if err != nil {
		return nil, err
	}

	return &clientConnUDPListener{
		cc:          cc,
		pc:          pc,
		frameBuffer: multibuffer.New(uint64(cc.conf.ReadBufferCount), uint64(cc.conf.ReadBufferSize)),
		lastFrameTime: func() *int64 {
			v := int64(0)
			return &v
		}(),
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
		buf := l.frameBuffer.Next()
		n, addr, err := l.pc.ReadFrom(buf)
		if err != nil {
			return
		}

		uaddr := addr.(*net.UDPAddr)

		if !l.remoteIP.Equal(uaddr.IP) || (l.remotePort != 0 && l.remotePort != uaddr.Port) {
			continue
		}

		now := time.Now()
		atomic.StoreInt64(l.lastFrameTime, now.Unix())
		l.cc.rtcpReceivers[l.trackID].ProcessFrame(now, l.streamType, buf[:n])
		l.cc.readCB(l.trackID, l.streamType, buf[:n])
	}
}

func (l *clientConnUDPListener) write(buf []byte) error {
	l.pc.SetWriteDeadline(time.Now().Add(l.cc.conf.WriteTimeout))
	_, err := l.pc.WriteTo(buf, &net.UDPAddr{
		IP:   l.remoteIP,
		Zone: l.remoteZone,
		Port: l.remotePort,
	})
	return err
}
