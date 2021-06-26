package gortsplib

import (
	"math/rand"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/net/ipv4"

	"github.com/aler9/gortsplib/pkg/multibuffer"
)

const (
	// use the same buffer size as gstreamer's rtspsrc
	clientConnUDPKernelReadBufferSize = 0x80000
)

type clientConnUDPListener struct {
	cc            *ClientConn
	pc            *net.UDPConn
	remoteReadIP  net.IP
	remoteWriteIP net.IP
	remoteZone    string
	remotePort    int
	trackID       int
	streamType    StreamType
	running       bool
	frameBuffer   *multibuffer.MultiBuffer
	lastFrameTime *int64
	writeMutex    sync.Mutex

	// out
	done chan struct{}
}

func newClientConnUDPListenerPair(cc *ClientConn) (*clientConnUDPListener, *clientConnUDPListener) {
	// choose two consecutive ports in range 65535-10000
	// rtp must be even and rtcp odd
	for {
		rtpPort := (rand.Intn((65535-10000)/2) * 2) + 10000
		rtpListener, err := newClientConnUDPListener(cc, false, ":"+strconv.FormatInt(int64(rtpPort), 10))
		if err != nil {
			continue
		}

		rtcpPort := rtpPort + 1
		rtcpListener, err := newClientConnUDPListener(cc, false, ":"+strconv.FormatInt(int64(rtcpPort), 10))
		if err != nil {
			rtpListener.close()
			continue
		}

		return rtpListener, rtcpListener
	}
}

func newClientConnUDPListener(cc *ClientConn, multicast bool, address string) (*clientConnUDPListener, error) {
	var pc *net.UDPConn
	if multicast {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}

		tmp, err := cc.c.ListenPacket("udp", "224.0.0.0:"+port)
		if err != nil {
			return nil, err
		}

		p := ipv4.NewPacketConn(tmp)

		err = p.SetTTL(127)
		if err != nil {
			return nil, err
		}

		intfs, err := net.Interfaces()
		if err != nil {
			return nil, err
		}

		for _, intf := range intfs {
			err := p.JoinGroup(&intf, &net.UDPAddr{IP: net.ParseIP(host)})
			if err != nil {
				return nil, err
			}
		}

		pc = tmp.(*net.UDPConn)
	} else {
		tmp, err := cc.c.ListenPacket("udp", address)
		if err != nil {
			return nil, err
		}
		pc = tmp.(*net.UDPConn)
	}

	err := pc.SetReadBuffer(clientConnUDPKernelReadBufferSize)
	if err != nil {
		return nil, err
	}

	return &clientConnUDPListener{
		cc:          cc,
		pc:          pc,
		frameBuffer: multibuffer.New(uint64(cc.c.ReadBufferCount), uint64(cc.c.ReadBufferSize)),
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

func (l *clientConnUDPListener) port() int {
	return l.pc.LocalAddr().(*net.UDPAddr).Port
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

	if l.cc.state == clientConnStatePlay {
		for {
			buf := l.frameBuffer.Next()
			n, addr, err := l.pc.ReadFrom(buf)
			if err != nil {
				return
			}

			uaddr := addr.(*net.UDPAddr)

			if !l.remoteReadIP.Equal(uaddr.IP) || (!isAnyPort(l.remotePort) && l.remotePort != uaddr.Port) {
				continue
			}

			now := time.Now()
			atomic.StoreInt64(l.lastFrameTime, now.Unix())
			l.cc.tracks[l.trackID].rtcpReceiver.ProcessFrame(now, l.streamType, buf[:n])
			l.cc.pullReadCB()(l.trackID, l.streamType, buf[:n])
		}
	} else { // record
		for {
			buf := l.frameBuffer.Next()
			n, addr, err := l.pc.ReadFrom(buf)
			if err != nil {
				return
			}

			uaddr := addr.(*net.UDPAddr)

			if !l.remoteReadIP.Equal(uaddr.IP) || (!isAnyPort(l.remotePort) && l.remotePort != uaddr.Port) {
				continue
			}

			now := time.Now()
			atomic.StoreInt64(l.lastFrameTime, now.Unix())
			l.cc.pullReadCB()(l.trackID, l.streamType, buf[:n])
		}
	}
}

func (l *clientConnUDPListener) write(buf []byte) error {
	l.writeMutex.Lock()
	defer l.writeMutex.Unlock()

	l.pc.SetWriteDeadline(time.Now().Add(l.cc.c.WriteTimeout))
	_, err := l.pc.WriteTo(buf, &net.UDPAddr{
		IP:   l.remoteWriteIP,
		Zone: l.remoteZone,
		Port: l.remotePort,
	})
	return err
}
