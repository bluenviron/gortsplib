package gortsplib

import (
	"crypto/rand"
	"math/big"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bluenviron/gortsplib/v4/pkg/multicast"
)

// Global registry for shared UDP listeners
var (
	sharedListenersMutex sync.Mutex
	sharedListeners      = make(map[string]*sharedUDPListener)
)

// sharedUDPListener represents a shared UDP listener that can be used by multiple clients
type sharedUDPListener struct {
	pc             packetConn
	address        string
	lastPacketTime *int64
	clientsMutex   sync.RWMutex
	clients        map[rtspServerAddr]readFunc
	refCount       int
	listening      bool
	done           chan struct{}
}

// newSharedUDPListener creates a new shared UDP listener
func newSharedUDPListener(address string, listenerFunc func(network, address string) (net.PacketConn, error)) (*sharedUDPListener, error) {
	tmp, err := listenerFunc("udp", address)
	if err != nil {
		return nil, err
	}
	pc := tmp.(*net.UDPConn)

	err = pc.SetReadBuffer(udpKernelReadBufferSize)
	if err != nil {
		pc.Close()
		return nil, err
	}

	s := &sharedUDPListener{
		pc:             pc,
		address:        address,
		lastPacketTime: int64Ptr(0),
		clients:        make(map[rtspServerAddr]readFunc),
		refCount:       1,
	}

	return s, nil
}

// start starts the shared listener
func (s *sharedUDPListener) start() {
	if !s.listening {
		s.listening = true
		s.pc.SetReadDeadline(time.Time{})
		s.done = make(chan struct{})
		go s.run()
	}
}

// stop stops the shared listener
func (s *sharedUDPListener) stop() {
	if s.listening {
		s.pc.SetReadDeadline(time.Now())
		<-s.done
		s.listening = false
	}
}

// run is the goroutine that reads packets from the UDP socket and dispatches them
func (s *sharedUDPListener) run() {
	defer close(s.done)

	var buf []byte

	createNewBuffer := func() {
		buf = make([]byte, udpMaxPayloadSize+1)
	}

	createNewBuffer()

	for {
		n, addr, err := s.pc.ReadFrom(buf)
		if err != nil {
			return
		}

		uaddr := addr.(*net.UDPAddr)

		now := time.Now()
		atomic.StoreInt64(s.lastPacketTime, now.Unix())

		func() {
			s.clientsMutex.RLock()
			defer s.clientsMutex.RUnlock()

			var ca rtspServerAddr
			ca.fill(uaddr.IP, uaddr.Port)
			cb, ok := s.clients[ca]
			if !ok {
				return
			}

			if cb(buf[:n]) {
				createNewBuffer()
			}
		}()
	}
}

// addClient adds a client to the shared listener
func (s *sharedUDPListener) addClient(ip net.IP, port int, cb readFunc) {
	var addr rtspServerAddr
	addr.fill(ip, port)

	s.clientsMutex.Lock()
	defer s.clientsMutex.Unlock()

	s.clients[addr] = cb
}

// removeClient removes a client from the shared listener
func (s *sharedUDPListener) removeClient(ip net.IP, port int) {
	var addr rtspServerAddr
	addr.fill(ip, port)

	s.clientsMutex.Lock()
	defer s.clientsMutex.Unlock()

	delete(s.clients, addr)
}

// port returns the local port number
func (s *sharedUDPListener) port() int {
	return s.pc.LocalAddr().(*net.UDPAddr).Port
}

// getOrCreateSharedListener gets existing or creates a new shared UDP listener
func getOrCreateSharedListener(address string, listenerFunc func(network, address string) (net.PacketConn, error)) (*sharedUDPListener, error) {
	sharedListenersMutex.Lock()
	defer sharedListenersMutex.Unlock()

	if listener, ok := sharedListeners[address]; ok {
		listener.refCount++
		return listener, nil
	}

	listener, err := newSharedUDPListener(address, listenerFunc)
	if err != nil {
		return nil, err
	}

	sharedListeners[address] = listener
	return listener, nil
}

// releaseSharedListener decreases reference count and cleans up if no more references
func releaseSharedListener(address string) {
	sharedListenersMutex.Lock()
	defer sharedListenersMutex.Unlock()

	listener, ok := sharedListeners[address]
	if !ok {
		return
	}

	listener.refCount--
	if listener.refCount <= 0 {
		listener.stop()
		listener.pc.Close()
		delete(sharedListeners, address)
	}
}

func int64Ptr(v int64) *int64 {
	return &v
}

func randInRange(maxVal int) (int, error) {
	b := big.NewInt(int64(maxVal + 1))
	n, err := rand.Int(rand.Reader, b)
	if err != nil {
		return 0, err
	}
	return int(n.Int64()), nil
}

func createUDPListenerPair(c *Client) (*clientUDPListener, *clientUDPListener, error) {
	var rtpPort, rtcpPort int

	// Use fixed ports if specified
	if c.ClientRTPPort != 0 && c.ClientRTCPPort != 0 {
		rtpPort = c.ClientRTPPort
		rtcpPort = c.ClientRTCPPort
	} else {
		// choose two consecutive ports in range 65535-10000
		// RTP port must be even and RTCP port odd
		for {
			v, err := randInRange((65535 - 10000) / 2)
			if err != nil {
				return nil, nil, err
			}

			rtpPort = v*2 + 10000
			rtcpPort = rtpPort + 1
			break
		}
	}

	rtpListener := &clientUDPListener{
		c:                 c,
		multicastEnable:   false,
		multicastSourceIP: nil,
		address:           net.JoinHostPort("", strconv.FormatInt(int64(rtpPort), 10)),
	}
	err := rtpListener.initialize()
	if err != nil {
		if c.ClientRTPPort != 0 {
			return nil, nil, err
		}
		// If using random ports, try again with another pair
		return createUDPListenerPair(c)
	}

	rtcpListener := &clientUDPListener{
		c:                 c,
		multicastEnable:   false,
		multicastSourceIP: nil,
		address:           net.JoinHostPort("", strconv.FormatInt(int64(rtcpPort), 10)),
	}
	err = rtcpListener.initialize()
	if err != nil {
		rtpListener.close()
		if c.ClientRTPPort != 0 {
			return nil, nil, err
		}
		// If using random ports, try again with another pair
		return createUDPListenerPair(c)
	}

	return rtpListener, rtcpListener, nil
}

type rtspServerAddr struct {
	ip   [net.IPv6len]byte // use a fixed-size array to enable the equality operator
	port int
}

func (p *rtspServerAddr) fill(ip net.IP, port int) {
	p.port = port

	if len(ip) == net.IPv4len {
		copy(p.ip[0:], []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0xff, 0xff}) // v4InV6Prefix
		copy(p.ip[12:], ip)
	} else {
		copy(p.ip[:], ip)
	}
}

type packetConn interface {
	net.PacketConn
	SetReadBuffer(int) error
}

type clientUDPListener struct {
	c                 *Client
	multicastEnable   bool
	multicastSourceIP net.IP
	address           string

	pc             packetConn
	readFunc       readFunc
	readIP         net.IP
	readPort       int
	writeAddr      *net.UDPAddr
	sharedListener *sharedUDPListener // Reference to a shared listener if using fixed ports

	running        bool
	lastPacketTime *int64

	// Support for multiple servers sharing the same UDP port
	clientsMutex sync.RWMutex
	clients      map[rtspServerAddr]readFunc

	done chan struct{}
}

func (u *clientUDPListener) initialize() error {
	// For fixed ports, use shared UDP listeners
	if u.c.ClientRTPPort != 0 && u.c.ClientRTCPPort != 0 {
		sharedListener, err := getOrCreateSharedListener(u.address, u.c.ListenPacket)
		if err != nil {
			return err
		}

		u.sharedListener = sharedListener
		u.pc = sharedListener.pc
		u.lastPacketTime = sharedListener.lastPacketTime
		return nil
	}

	// Otherwise, use the original implementation for dynamic ports
	if u.multicastEnable {
		intf, err := multicast.InterfaceForSource(u.multicastSourceIP)
		if err != nil {
			return err
		}

		u.pc, err = multicast.NewSingleConn(intf, u.address, u.c.ListenPacket)
		if err != nil {
			return err
		}
	} else {
		tmp, err := u.c.ListenPacket(restrictNetwork("udp", u.address))
		if err != nil {
			return err
		}
		u.pc = tmp.(*net.UDPConn)
	}

	err := u.pc.SetReadBuffer(udpKernelReadBufferSize)
	if err != nil {
		u.pc.Close()
		return err
	}

	u.lastPacketTime = int64Ptr(0)
	u.clients = make(map[rtspServerAddr]readFunc)
	return nil
}

func (u *clientUDPListener) addServer(ip net.IP, port int, cb readFunc) {
	// Support for AnyPortEnable
	u.readIP = ip
	u.readFunc = cb

	if u.c.ClientRTPPort != 0 && u.c.ClientRTCPPort != 0 && u.sharedListener != nil {
		// When using fixed ports, add the client to the shared listener
		u.sharedListener.addClient(ip, port, cb)
	} else {
		// For non-fixed ports, use the original implementation
		var addr rtspServerAddr
		addr.fill(ip, port)

		u.clientsMutex.Lock()
		defer u.clientsMutex.Unlock()

		u.clients[addr] = cb
	}
}

func (u *clientUDPListener) removeServer(ip net.IP, port int) {
	if u.c.ClientRTPPort != 0 && u.c.ClientRTCPPort != 0 && u.sharedListener != nil {
		// When using fixed ports, remove the client from the shared listener
		u.sharedListener.removeClient(ip, port)
	} else {
		// For non-fixed ports, use the original implementation
		var addr rtspServerAddr
		addr.fill(ip, port)

		u.clientsMutex.Lock()
		defer u.clientsMutex.Unlock()

		delete(u.clients, addr)
	}
}

func (u *clientUDPListener) close() {
	if u.running {
		u.stop()
	}

	if u.sharedListener != nil {
		// Release reference to the shared listener
		releaseSharedListener(u.address)
	} else {
		// Original implementation for non-shared listener
		u.pc.Close()
	}
}

func (u *clientUDPListener) port() int {
	return u.pc.LocalAddr().(*net.UDPAddr).Port
}

func (u *clientUDPListener) start() {
	u.running = true

	if u.sharedListener != nil {
		// Use the shared listener's start method
		u.sharedListener.start()
	} else {
		// Original implementation
		u.pc.SetReadDeadline(time.Time{})
		u.done = make(chan struct{})
		go u.run()
	}
}

func (u *clientUDPListener) stop() {
	if u.sharedListener != nil {
		// For shared listeners, we don't stop them here as they're managed by the registry
		u.running = false
	} else {
		// Original implementation
		u.pc.SetReadDeadline(time.Now())
		<-u.done
		u.running = false
	}
}

func (u *clientUDPListener) run() {
	defer close(u.done)

	var buf []byte

	createNewBuffer := func() {
		buf = make([]byte, udpMaxPayloadSize+1)
	}

	createNewBuffer()

	for {
		n, addr, err := u.pc.ReadFrom(buf)
		if err != nil {
			return
		}

		uaddr := addr.(*net.UDPAddr)

		now := u.c.timeNow()
		atomic.StoreInt64(u.lastPacketTime, now.Unix())

		// For dynamic ports with multiple client support
		if u.c.ClientRTPPort != 0 && u.c.ClientRTCPPort != 0 {
			func() {
				u.clientsMutex.RLock()
				defer u.clientsMutex.RUnlock()

				var ca rtspServerAddr
				ca.fill(uaddr.IP, uaddr.Port)
				cb, ok := u.clients[ca]
				if !ok {
					return
				}

				if cb(buf[:n]) {
					createNewBuffer()
				}
			}()
		} else {
			// Legacy handling with AnyPortEnable
			if !u.readIP.Equal(uaddr.IP) {
				continue
			}

			// in case of anyPortEnable, store the port of the first packet we receive.
			// this reduces security issues
			if u.c.AnyPortEnable && u.readPort == 0 {
				u.readPort = uaddr.Port
			} else if u.readPort != uaddr.Port {
				continue
			}

			if u.readFunc(buf[:n]) {
				createNewBuffer()
			}
		}
	}
}

func (u *clientUDPListener) write(payload []byte) error {
	// no mutex is needed here since Write() has an internal lock.
	// https://github.com/golang/go/issues/27203#issuecomment-534386117
	u.pc.SetWriteDeadline(time.Now().Add(u.c.WriteTimeout))
	_, err := u.pc.WriteTo(payload, u.writeAddr)
	return err
}
