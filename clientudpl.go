package gortsplib

import (
	"crypto/rand"
	"net"
	"strconv"
	"sync/atomic"
	"time"

	"golang.org/x/net/ipv4"

	"github.com/aler9/gortsplib/pkg/multibuffer"
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
	c              *Client
	pc             *net.UDPConn
	remoteReadIP   net.IP
	remoteWriteIP  net.IP
	remoteZone     string
	remotePort     int
	trackID        int
	isRTP          bool
	running        bool
	readBuffer     *multibuffer.MultiBuffer
	lastPacketTime *int64
	processFunc    func(time.Time, []byte)

	readerDone chan struct{}
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
		c:          c,
		pc:         pc,
		readBuffer: multibuffer.New(uint64(c.ReadBufferCount), uint64(c.ReadBufferSize)),
		lastPacketTime: func() *int64 {
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
	if l.c.state == clientStatePlay {
		if l.isRTP {
			l.processFunc = l.processPlayRTP
		} else {
			l.processFunc = l.processPlayRTCP
		}
	} else {
		l.processFunc = l.processRecord
	}

	l.running = true
	l.pc.SetReadDeadline(time.Time{})
	l.readerDone = make(chan struct{})
	go l.runReader()
}

func (l *clientUDPListener) stop() {
	l.pc.SetReadDeadline(time.Now())
	<-l.readerDone
}

func (l *clientUDPListener) runReader() {
	defer close(l.readerDone)

	for {
		buf := l.readBuffer.Next()
		n, addr, err := l.pc.ReadFrom(buf)
		if err != nil {
			return
		}

		uaddr := addr.(*net.UDPAddr)

		if !l.remoteReadIP.Equal(uaddr.IP) || (!l.c.AnyPortEnable && l.remotePort != uaddr.Port) {
			continue
		}

		now := time.Now()
		atomic.StoreInt64(l.lastPacketTime, now.Unix())

		l.processFunc(now, buf[:n])
	}
}

func (l *clientUDPListener) processPlayRTP(now time.Time, payload []byte) {
	l.c.tracks[l.trackID].rtcpReceiver.ProcessPacketRTP(now, payload)
	l.c.OnPacketRTP(l.trackID, payload)
}

func (l *clientUDPListener) processPlayRTCP(now time.Time, payload []byte) {
	l.c.tracks[l.trackID].rtcpReceiver.ProcessPacketRTCP(now, payload)
	l.c.OnPacketRTCP(l.trackID, payload)
}

func (l *clientUDPListener) processRecord(now time.Time, payload []byte) {
	l.c.OnPacketRTCP(l.trackID, payload)
}

func (l *clientUDPListener) write(buf []byte) error {
	// no mutex is needed here since Write() has an internal lock.
	// https://github.com/golang/go/issues/27203#issuecomment-534386117

	l.pc.SetWriteDeadline(time.Now().Add(l.c.WriteTimeout))
	_, err := l.pc.WriteTo(buf, &net.UDPAddr{
		IP:   l.remoteWriteIP,
		Zone: l.remoteZone,
		Port: l.remotePort,
	})
	return err
}
