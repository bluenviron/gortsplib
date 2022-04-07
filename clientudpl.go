package gortsplib

import (
	"crypto/rand"
	"net"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/pion/rtcp"
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
	c               *Client
	pc              *net.UDPConn
	remoteReadIP    net.IP
	remoteReadPort  int
	remoteWriteAddr *net.UDPAddr
	trackID         int
	isRTP           bool
	running         bool
	readBuffer      *multibuffer.MultiBuffer
	rtpPacketBuffer *rtpPacketMultiBuffer
	lastPacketTime  *int64
	processFunc     func(time.Time, []byte)

	readerDone chan struct{}
}

func newClientUDPListenerPair(c *Client) (*clientUDPListener, *clientUDPListener) {
	// choose two consecutive ports in range 65535-10000
	// RTP port must be even and RTCP port odd
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
		c:               c,
		pc:              pc,
		readBuffer:      multibuffer.New(uint64(c.ReadBufferCount), uint64(c.ReadBufferSize)),
		rtpPacketBuffer: newRTPPacketMultiBuffer(uint64(c.ReadBufferCount)),
		lastPacketTime: func() *int64 {
			v := int64(0)
			return &v
		}(),
	}, nil
}

func (u *clientUDPListener) close() {
	if u.running {
		u.stop()
	}
	u.pc.Close()
}

func (u *clientUDPListener) port() int {
	return u.pc.LocalAddr().(*net.UDPAddr).Port
}

func (u *clientUDPListener) start(forPlay bool) {
	if forPlay {
		if u.isRTP {
			u.processFunc = u.processPlayRTP
		} else {
			u.processFunc = u.processPlayRTCP
		}
	} else {
		u.processFunc = u.processRecordRTCP
	}

	u.running = true
	u.pc.SetReadDeadline(time.Time{})
	u.readerDone = make(chan struct{})
	go u.runReader()
}

func (u *clientUDPListener) stop() {
	u.pc.SetReadDeadline(time.Now())
	<-u.readerDone
}

func (u *clientUDPListener) runReader() {
	defer close(u.readerDone)

	for {
		buf := u.readBuffer.Next()
		n, addr, err := u.pc.ReadFrom(buf)
		if err != nil {
			return
		}

		uaddr := addr.(*net.UDPAddr)

		if !u.remoteReadIP.Equal(uaddr.IP) || (!u.c.AnyPortEnable && u.remoteReadPort != uaddr.Port) {
			continue
		}

		now := time.Now()
		atomic.StoreInt64(u.lastPacketTime, now.Unix())

		u.processFunc(now, buf[:n])
	}
}

func (u *clientUDPListener) processPlayRTP(now time.Time, payload []byte) {
	pkt := u.rtpPacketBuffer.next()
	err := pkt.Unmarshal(payload)
	if err != nil {
		return
	}

	u.c.onPacketRTP(u.trackID, pkt)
}

func (u *clientUDPListener) processPlayRTCP(now time.Time, payload []byte) {
	packets, err := rtcp.Unmarshal(payload)
	if err != nil {
		return
	}

	for _, pkt := range packets {
		u.c.tracks[u.trackID].rtcpReceiver.ProcessPacketRTCP(now, pkt)
		u.c.onPacketRTCP(u.trackID, pkt)
	}
}

func (u *clientUDPListener) processRecordRTCP(now time.Time, payload []byte) {
	packets, err := rtcp.Unmarshal(payload)
	if err != nil {
		return
	}

	for _, pkt := range packets {
		u.c.onPacketRTCP(u.trackID, pkt)
	}
}

func (u *clientUDPListener) write(payload []byte) error {
	// no mutex is needed here since Write() has an internal lock.
	// https://github.com/golang/go/issues/27203#issuecomment-534386117

	u.pc.SetWriteDeadline(time.Now().Add(u.c.WriteTimeout))
	_, err := u.pc.WriteTo(payload, u.remoteWriteAddr)
	return err
}
