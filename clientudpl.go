package gortsplib

import (
	"crypto/rand"
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
	clientUDPKernelReadBufferSize = 0x80000
)

func randUint32() uint32 {
	var b [4]byte
	rand.Read(b[:])
	return uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3])
}

func randIntn(n int) int {
	return int(randUint32() & (uint32(n) - 1))
}

type clientUDPListener struct {
	c             *Client
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

func newClientUDPListenerPair(c *Client) (*clientUDPListener, *clientUDPListener) {
	// choose two consecutive ports in range 65535-10000
	// rtp must be even and rtcp odd
	for {
		rtpPort := (randIntn((65535-10000)/2) * 2) + 10000
		rtpListener, err := newClientUDPListener(c, false, ":"+strconv.FormatInt(int64(rtpPort), 10))
		if err != nil {
			continue
		}

		rtcpPort := rtpPort + 1
		rtcpListener, err := newClientUDPListener(c, false, ":"+strconv.FormatInt(int64(rtcpPort), 10))
		if err != nil {
			rtpListener.close()
			continue
		}

		return rtpListener, rtcpListener
	}
}

func newClientUDPListener(c *Client, multicast bool, address string) (*clientUDPListener, error) {
	var pc *net.UDPConn
	if multicast {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}

		tmp, err := c.ListenPacket("udp", "224.0.0.0:"+port)
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
		tmp, err := c.ListenPacket("udp", address)
		if err != nil {
			return nil, err
		}
		pc = tmp.(*net.UDPConn)
	}

	err := pc.SetReadBuffer(clientUDPKernelReadBufferSize)
	if err != nil {
		return nil, err
	}

	return &clientUDPListener{
		c:           c,
		pc:          pc,
		frameBuffer: multibuffer.New(uint64(c.ReadBufferCount), uint64(c.ReadBufferSize)),
		lastFrameTime: func() *int64 {
			v := int64(0)
			return &v
		}(),
	}, nil
}

func (l *clientUDPListener) close() {
	if l.running {
		l.stop()
	}
	l.pc.Close()
}

func (l *clientUDPListener) port() int {
	return l.pc.LocalAddr().(*net.UDPAddr).Port
}

func (l *clientUDPListener) start() {
	l.running = true
	l.pc.SetReadDeadline(time.Time{})
	l.done = make(chan struct{})
	go l.run()
}

func (l *clientUDPListener) stop() {
	l.pc.SetReadDeadline(time.Now())
	<-l.done
}

func (l *clientUDPListener) run() {
	defer close(l.done)

	if l.c.state == clientStatePlay {
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

			if l.streamType == StreamTypeRTP {
				l.c.tracks[l.trackID].rtcpReceiver.ProcessPacketRTP(now, buf[:n])
				l.c.OnPacketRTP(l.c, l.trackID, buf[:n])
			} else {
				l.c.tracks[l.trackID].rtcpReceiver.ProcessPacketRTCP(now, buf[:n])
				l.c.OnPacketRTCP(l.c, l.trackID, buf[:n])
			}
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
			l.c.OnPacketRTCP(l.c, l.trackID, buf[:n])
		}
	}
}

func (l *clientUDPListener) write(buf []byte) error {
	l.writeMutex.Lock()
	defer l.writeMutex.Unlock()

	l.pc.SetWriteDeadline(time.Now().Add(l.c.WriteTimeout))
	_, err := l.pc.WriteTo(buf, &net.UDPAddr{
		IP:   l.remoteWriteIP,
		Zone: l.remoteZone,
		Port: l.remotePort,
	})
	return err
}
