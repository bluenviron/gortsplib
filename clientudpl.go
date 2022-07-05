package gortsplib

import (
	"crypto/rand"
	"net"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/pion/rtcp"
	"golang.org/x/net/ipv4"
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
	c     *Client
	pc    *net.UDPConn
	ct    *clientTrack
	isRTP bool

	readIP    net.IP
	readPort  int
	writeAddr *net.UDPAddr

	running        bool
	lastPacketTime *int64

	readerDone chan struct{}
}

func newClientUDPListenerPair(c *Client, ct *clientTrack) (*clientUDPListener, *clientUDPListener) {
	// choose two consecutive ports in range 65535-10000
	// RTP port must be even and RTCP port odd
	for {
		rtpPort := (randIntn((65535-10000)/2) * 2) + 10000
		rtpListener, err := newClientUDPListener(
			c,
			false,
			":"+strconv.FormatInt(int64(rtpPort), 10),
			ct,
			true)
		if err != nil {
			continue
		}

		rtcpPort := rtpPort + 1
		rtcpListener, err := newClientUDPListener(
			c,
			false,
			":"+strconv.FormatInt(int64(rtcpPort), 10),
			ct,
			false)
		if err != nil {
			rtpListener.close()
			continue
		}

		return rtpListener, rtcpListener
	}
}

func newClientUDPListener(
	c *Client,
	multicast bool,
	address string,
	ct *clientTrack,
	isRTP bool,
) (*clientUDPListener, error) {
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

		err = p.SetMulticastTTL(multicastTTL)
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

	err := pc.SetReadBuffer(udpKernelReadBufferSize)
	if err != nil {
		return nil, err
	}

	return &clientUDPListener{
		c:     c,
		pc:    pc,
		ct:    ct,
		isRTP: isRTP,
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
	u.running = true
	u.pc.SetReadDeadline(time.Time{})
	u.readerDone = make(chan struct{})
	go u.runReader(forPlay)
}

func (u *clientUDPListener) stop() {
	u.pc.SetReadDeadline(time.Now())
	<-u.readerDone
}

func (u *clientUDPListener) runReader(forPlay bool) {
	defer close(u.readerDone)

	var processFunc func(time.Time, []byte)
	if forPlay {
		if u.isRTP {
			processFunc = u.processPlayRTP
		} else {
			processFunc = u.processPlayRTCP
		}
	} else {
		processFunc = u.processRecordRTCP
	}

	for {
		buf := make([]byte, maxPacketSize)
		n, addr, err := u.pc.ReadFrom(buf)
		if err != nil {
			return
		}

		uaddr := addr.(*net.UDPAddr)

		if !u.readIP.Equal(uaddr.IP) || (!u.c.AnyPortEnable && u.readPort != uaddr.Port) {
			continue
		}

		now := time.Now()
		atomic.StoreInt64(u.lastPacketTime, now.Unix())

		processFunc(now, buf[:n])
	}
}

func (u *clientUDPListener) processPlayRTP(now time.Time, payload []byte) {
	pkt := u.ct.udpRTPPacketBuffer.next()
	err := pkt.Unmarshal(payload)
	if err != nil {
		return
	}

	packets := u.ct.reorderer.Process(pkt)

	for _, pkt := range packets {
		out, err := u.ct.cleaner.Process(pkt)
		if err != nil {
			return
		}
		out0 := out[0]

		u.ct.udpRTCPReceiver.ProcessPacketRTP(time.Now(), pkt, out0.PTSEqualsDTS)

		u.c.OnPacketRTP(&ClientOnPacketRTPCtx{
			TrackID:      u.ct.id,
			Packet:       out0.Packet,
			PTSEqualsDTS: out0.PTSEqualsDTS,
			H264NALUs:    out0.H264NALUs,
			H264PTS:      out0.H264PTS,
		})
	}
}

func (u *clientUDPListener) processPlayRTCP(now time.Time, payload []byte) {
	packets, err := rtcp.Unmarshal(payload)
	if err != nil {
		return
	}

	for _, pkt := range packets {
		u.ct.udpRTCPReceiver.ProcessPacketRTCP(now, pkt)
		u.c.OnPacketRTCP(&ClientOnPacketRTCPCtx{
			TrackID: u.ct.id,
			Packet:  pkt,
		})
	}
}

func (u *clientUDPListener) processRecordRTCP(now time.Time, payload []byte) {
	packets, err := rtcp.Unmarshal(payload)
	if err != nil {
		return
	}

	for _, pkt := range packets {
		u.c.OnPacketRTCP(&ClientOnPacketRTCPCtx{
			TrackID: u.ct.id,
			Packet:  pkt,
		})
	}
}

func (u *clientUDPListener) write(payload []byte) error {
	// no mutex is needed here since Write() has an internal lock.
	// https://github.com/golang/go/issues/27203#issuecomment-534386117

	u.pc.SetWriteDeadline(time.Now().Add(u.c.WriteTimeout))
	_, err := u.pc.WriteTo(payload, u.writeAddr)
	return err
}
