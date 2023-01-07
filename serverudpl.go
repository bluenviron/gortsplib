package gortsplib

import (
	"net"
	"strconv"
	"sync"
	"time"

	"golang.org/x/net/ipv4"
)

func serverFindFormatWithSSRC(
	formats map[uint8]*serverSessionFormat,
	ssrc uint32,
) *serverSessionFormat {
	for _, format := range formats {
		tssrc, ok := format.udpRTCPReceiver.LastSSRC()
		if ok && tssrc == ssrc {
			return format
		}
	}
	return nil
}

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

func onDecodeError(ss *ServerSession, err error) {
	if h, ok := ss.s.Handler.(ServerHandlerOnDecodeError); ok {
		h.OnDecodeError(&ServerHandlerOnDecodeErrorCtx{
			Session: ss,
			Error:   err,
		})
	}
}

type serverUDPListener struct {
	pc           *net.UDPConn
	listenIP     net.IP
	isRTP        bool
	writeTimeout time.Duration
	clientsMutex sync.RWMutex
	clients      map[clientAddr]*serverSessionMedia

	readerDone chan struct{}
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
		ip.String()+":"+strconv.FormatInt(int64(multicastRTPPort), 10),
		true,
	)
	if err != nil {
		return nil, nil, err
	}

	rtcpl, err := newServerUDPListener(
		listenPacket,
		writeTimeout,
		true,
		ip.String()+":"+strconv.FormatInt(int64(multicastRTCPPort), 10),
		false,
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
	multicast bool,
	address string,
	isRTP bool,
) (*serverUDPListener, error) {
	var pc *net.UDPConn
	var listenIP net.IP
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

		listenIP = net.ParseIP(host)

		for _, intf := range intfs {
			if (intf.Flags & net.FlagMulticast) != 0 {
				// do not check for errors.
				// on macOS, there are interfaces with the multicast flag but
				// without support for multicast, that makes this function fail.
				p.JoinGroup(&intf, &net.UDPAddr{IP: listenIP})
			}
		}

		pc = tmp.(*net.UDPConn)
	} else {
		tmp, err := listenPacket("udp", address)
		if err != nil {
			return nil, err
		}

		pc = tmp.(*net.UDPConn)
		listenIP = tmp.LocalAddr().(*net.UDPAddr).IP
	}

	err := pc.SetReadBuffer(udpKernelReadBufferSize)
	if err != nil {
		return nil, err
	}

	u := &serverUDPListener{
		pc:           pc,
		listenIP:     listenIP,
		clients:      make(map[clientAddr]*serverSessionMedia),
		isRTP:        isRTP,
		writeTimeout: writeTimeout,
		readerDone:   make(chan struct{}),
	}

	go u.runReader()

	return u, nil
}

func (u *serverUDPListener) close() {
	u.pc.Close()
	<-u.readerDone
}

func (u *serverUDPListener) ip() net.IP {
	return u.listenIP
}

func (u *serverUDPListener) port() int {
	return u.pc.LocalAddr().(*net.UDPAddr).Port
}

func (u *serverUDPListener) runReader() {
	defer close(u.readerDone)

	var readFunc func(*serverSessionMedia, []byte)
	if u.isRTP {
		readFunc = func(sm *serverSessionMedia, payload []byte) {
			sm.readRTP(payload)
		}
	} else {
		readFunc = func(sm *serverSessionMedia, payload []byte) {
			sm.readRTCP(payload)
		}
	}

	for {
		buf := make([]byte, maxPacketSize+1)
		n, addr, err := u.pc.ReadFromUDP(buf)
		if err != nil {
			break
		}

		func() {
			u.clientsMutex.RLock()
			defer u.clientsMutex.RUnlock()

			var clientAddr clientAddr
			clientAddr.fill(addr.IP, addr.Port)
			sm, ok := u.clients[clientAddr]
			if !ok {
				return
			}

			readFunc(sm, buf[:n])
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

func (u *serverUDPListener) addClient(ip net.IP, port int, sm *serverSessionMedia) {
	u.clientsMutex.Lock()
	defer u.clientsMutex.Unlock()

	var addr clientAddr
	addr.fill(ip, port)

	u.clients[addr] = sm
}

func (u *serverUDPListener) removeClient(sm *serverSessionMedia) {
	u.clientsMutex.Lock()
	defer u.clientsMutex.Unlock()

	for addr, sm1 := range u.clients {
		if sm1 == sm {
			delete(u.clients, addr)
		}
	}
}
