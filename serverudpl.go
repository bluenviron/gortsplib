package gortsplib

import (
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/aler9/gortsplib/pkg/multibuffer"
)

const (
	// use the same buffer size as gstreamer's rtspsrc
	kernelReadBufferSize = 0x80000

	readBufferSize = 2048
)

type publisherData struct {
	publisher *ServerConn
	trackID   int
}

type publisherAddr struct {
	ip   [net.IPv6len]byte // use a fixed-size array to enable the equality operator
	port int
}

func (p *publisherAddr) fill(ip net.IP, port int) {
	p.port = port

	if len(ip) == net.IPv4len {
		copy(p.ip[0:], []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0xff, 0xff}) // v4InV6Prefix
		copy(p.ip[12:], ip)
	} else {
		copy(p.ip[:], ip)
	}
}

// ServerUDPListener is a UDP server that can be used to send and receive RTP and RTCP packets.
type ServerUDPListener struct {
	streamType StreamType

	pc              *net.UDPConn
	readBuf         *multibuffer.MultiBuffer
	publishersMutex sync.RWMutex
	publishers      map[publisherAddr]*publisherData
	writeMutex      sync.Mutex

	// out
	done chan struct{}
}

// NewServerUDPListener allocates a ServerUDPListener.
func NewServerUDPListener(address string) (*ServerUDPListener, error) {
	tmp, err := net.ListenPacket("udp", address)
	if err != nil {
		return nil, err
	}
	pc := tmp.(*net.UDPConn)

	err = pc.SetReadBuffer(kernelReadBufferSize)
	if err != nil {
		return nil, err
	}

	s := &ServerUDPListener{
		pc:         pc,
		readBuf:    multibuffer.New(1, readBufferSize),
		publishers: make(map[publisherAddr]*publisherData),
		done:       make(chan struct{}),
	}

	go s.run()

	return s, nil
}

// Close closes the listener.
func (s *ServerUDPListener) Close() {
	s.pc.Close()
	<-s.done
}

func (s *ServerUDPListener) run() {
	defer close(s.done)

	for {
		buf := s.readBuf.Next()
		n, addr, err := s.pc.ReadFromUDP(buf)
		if err != nil {
			break
		}

		func() {
			s.publishersMutex.RLock()
			defer s.publishersMutex.RUnlock()

			// find publisher data
			var pubAddr publisherAddr
			pubAddr.fill(addr.IP, addr.Port)
			pubData, ok := s.publishers[pubAddr]
			if !ok {
				return
			}

			now := time.Now()
			atomic.StoreInt64(pubData.publisher.udpLastFrameTimes[pubData.trackID], now.Unix())
			pubData.publisher.rtcpReceivers[pubData.trackID].ProcessFrame(now, s.streamType, buf[:n])
			pubData.publisher.readHandlers.OnFrame(pubData.trackID, s.streamType, buf[:n])
		}()
	}
}

func (s *ServerUDPListener) port() int {
	return s.pc.LocalAddr().(*net.UDPAddr).Port
}

func (s *ServerUDPListener) write(writeTimeout time.Duration, buf []byte, addr *net.UDPAddr) error {
	s.writeMutex.Lock()
	defer s.writeMutex.Unlock()

	s.pc.SetWriteDeadline(time.Now().Add(writeTimeout))
	_, err := s.pc.WriteTo(buf, addr)
	return err
}

func (s *ServerUDPListener) addPublisher(ip net.IP, port int, trackID int, sc *ServerConn) {
	s.publishersMutex.Lock()
	defer s.publishersMutex.Unlock()

	var addr publisherAddr
	addr.fill(ip, port)

	s.publishers[addr] = &publisherData{
		publisher: sc,
		trackID:   trackID,
	}
}

func (s *ServerUDPListener) removePublisher(ip net.IP, port int) {
	s.publishersMutex.Lock()
	defer s.publishersMutex.Unlock()

	var addr publisherAddr
	addr.fill(ip, port)

	delete(s.publishers, addr)
}
