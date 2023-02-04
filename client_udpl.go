package gortsplib

import (
	"crypto/rand"
	"math/big"
	"net"
	"strconv"
	"sync/atomic"
	"time"

	"golang.org/x/net/ipv4"
)

func randInRange(max int) int {
	b := big.NewInt(int64(max + 1))
	n, _ := rand.Int(rand.Reader, b)
	return int(n.Int64())
}

type clientUDPListener struct {
	anyPortEnable bool
	writeTimeout  time.Duration
	pc            *net.UDPConn
	cm            *clientMedia
	isRTP         bool

	readIP    net.IP
	readPort  int
	writeAddr *net.UDPAddr

	running        bool
	lastPacketTime *int64

	readerDone chan struct{}
}

func newClientUDPListenerPair(
	listenPacket func(network, address string) (net.PacketConn, error),
	anyPortEnable bool,
	writeTimeout time.Duration,
	cm *clientMedia,
) (*clientUDPListener, *clientUDPListener) {
	// choose two consecutive ports in range 65535-10000
	// RTP port must be even and RTCP port odd
	for {
		rtpPort := randInRange((65535-10000)/2)*2 + 10000
		rtpListener, err := newClientUDPListener(
			listenPacket,
			anyPortEnable,
			writeTimeout,
			false,
			":"+strconv.FormatInt(int64(rtpPort), 10),
			cm,
			true)
		if err != nil {
			continue
		}

		rtcpPort := rtpPort + 1
		rtcpListener, err := newClientUDPListener(
			listenPacket,
			anyPortEnable,
			writeTimeout,
			false,
			":"+strconv.FormatInt(int64(rtcpPort), 10),
			cm,
			false)
		if err != nil {
			rtpListener.close()
			continue
		}

		return rtpListener, rtcpListener
	}
}

func newClientUDPListener(
	listenPacket func(network, address string) (net.PacketConn, error),
	anyPortEnable bool,
	writeTimeout time.Duration,
	multicast bool,
	address string,
	cm *clientMedia,
	isRTP bool,
) (*clientUDPListener, error) {
	var pc *net.UDPConn
	if multicast {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}

		tmp, err := listenPacket("udp", "224.0.0.0:"+port)
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
		tmp, err := listenPacket("udp", address)
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
		anyPortEnable: anyPortEnable,
		writeTimeout:  writeTimeout,
		pc:            pc,
		cm:            cm,
		isRTP:         isRTP,
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

	var readFunc func([]byte) error
	if u.isRTP {
		readFunc = u.cm.readRTP
	} else {
		readFunc = u.cm.readRTCP
	}

	for {
		buf := make([]byte, maxPacketSize+1)
		n, addr, err := u.pc.ReadFrom(buf)
		if err != nil {
			return
		}

		uaddr := addr.(*net.UDPAddr)

		if !u.readIP.Equal(uaddr.IP) {
			continue
		}

		// in case of anyPortEnable, store the port of the first packet we receive.
		// this reduces security issues
		if u.anyPortEnable && u.readPort == 0 {
			u.readPort = uaddr.Port
		} else if u.readPort != uaddr.Port {
			continue
		}

		now := time.Now()
		atomic.StoreInt64(u.lastPacketTime, now.Unix())

		readFunc(buf[:n])
	}
}

func (u *clientUDPListener) write(payload []byte) error {
	// no mutex is needed here since Write() has an internal lock.
	// https://github.com/golang/go/issues/27203#issuecomment-534386117
	u.pc.SetWriteDeadline(time.Now().Add(u.writeTimeout))
	_, err := u.pc.WriteTo(payload, u.writeAddr)
	return err
}
