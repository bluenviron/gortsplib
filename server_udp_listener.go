package gortsplib

import (
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/bluenviron/gortsplib/v4/pkg/multicast"
)

type clientAddr struct {
	ip   [net.IPv6len]byte // use a fixed-size array to enable the equality operator
	port int
}

func (p *clientAddr) fill(ip net.IP, port int) {
	p.port = port

	if len(ip) == net.IPv4len {
		copy(p.ip[0:], []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0xff, 0xff}) // v4InV6Prefix
		copy(p.ip[12:], ip)
	} else {
		copy(p.ip[:], ip)
	}
}

type serverUDPListener struct {
	pc           net.PacketConn
	listenIP     net.IP
	writeTimeout time.Duration
	clientsMutex sync.RWMutex
	clients      map[clientAddr]readFunc

	done chan struct{}
}

func newServerUDPListenerMulticastPair(
	listenPacket func(network, address string) (net.PacketConn, error),
	writeTimeout time.Duration,
	multicastRTPPort int,
	multicastRTCPPort int,
	ip net.IP,
) (*serverUDPListener, *serverUDPListener, error) {
	rtpl, err := newServerUDPListener(
		listenPacket,
		writeTimeout,
		true,
		net.JoinHostPort(ip.String(), strconv.FormatInt(int64(multicastRTPPort), 10)),
	)
	if err != nil {
		return nil, nil, err
	}

	rtcpl, err := newServerUDPListener(
		listenPacket,
		writeTimeout,
		true,
		net.JoinHostPort(ip.String(), strconv.FormatInt(int64(multicastRTCPPort), 10)),
	)
	if err != nil {
		rtpl.close()
		return nil, nil, err
	}

	return rtpl, rtcpl, nil
}

func newServerUDPListener(
	listenPacket func(network, address string) (net.PacketConn, error),
	writeTimeout time.Duration,
	multicastEnable bool,
	address string,
) (*serverUDPListener, error) {
	var pc packetConn
	var listenIP net.IP
	if multicastEnable {
		var err error
		pc, err = multicast.NewMultiConn(address, false, listenPacket)
		if err != nil {
			return nil, err
		}

		host, _, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}
		listenIP = net.ParseIP(host)
	} else {
		tmp, err := listenPacket(restrictNetwork("udp", address))
		if err != nil {
			return nil, err
		}
		pc = tmp.(*net.UDPConn)
		listenIP = tmp.LocalAddr().(*net.UDPAddr).IP
	}

	err := pc.SetReadBuffer(udpKernelReadBufferSize)
	if err != nil {
		pc.Close()
		return nil, err
	}

	u := &serverUDPListener{
		pc:           pc,
		listenIP:     listenIP,
		clients:      make(map[clientAddr]readFunc),
		writeTimeout: writeTimeout,
		done:         make(chan struct{}),
	}

	go u.run()

	return u, nil
}

func (u *serverUDPListener) close() {
	u.pc.Close()
	<-u.done
}

func (u *serverUDPListener) ip() net.IP {
	return u.listenIP
}

func (u *serverUDPListener) port() int {
	return u.pc.LocalAddr().(*net.UDPAddr).Port
}

func (u *serverUDPListener) run() {
	defer close(u.done)

	for {
		buf := make([]byte, udpMaxPayloadSize+1)
		n, addr2, err := u.pc.ReadFrom(buf)
		if err != nil {
			break
		}
		addr := addr2.(*net.UDPAddr)

		func() {
			u.clientsMutex.RLock()
			defer u.clientsMutex.RUnlock()

			var clientAddr clientAddr
			clientAddr.fill(addr.IP, addr.Port)
			cb, ok := u.clients[clientAddr]
			if !ok {
				return
			}

			cb(buf[:n])
		}()
	}
}

func (u *serverUDPListener) write(buf []byte, addr *net.UDPAddr) error {
	// no mutex is needed here since Write() has an internal lock.
	// https://github.com/golang/go/issues/27203#issuecomment-534386117
	u.pc.SetWriteDeadline(time.Now().Add(u.writeTimeout))
	_, err := u.pc.WriteTo(buf, addr)
	return err
}

func (u *serverUDPListener) addClient(ip net.IP, port int, cb readFunc) {
	var addr clientAddr
	addr.fill(ip, port)

	u.clientsMutex.Lock()
	defer u.clientsMutex.Unlock()

	u.clients[addr] = cb
}

func (u *serverUDPListener) removeClient(ip net.IP, port int) {
	var addr clientAddr
	addr.fill(ip, port)

	u.clientsMutex.Lock()
	defer u.clientsMutex.Unlock()

	delete(u.clients, addr)
}
