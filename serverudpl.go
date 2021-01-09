package gortsplib

import (
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/aler9/gortsplib/pkg/multibuffer"
	"github.com/aler9/gortsplib/pkg/ringbuffer"
)

const (
	serverConnUDPListenerKernelReadBufferSize = 0x80000 // same as gstreamer's rtspsrc
	serverConnUDPListenerReadBufferSize       = 2048
)

type bufAddrPair struct {
	buf  []byte
	addr *net.UDPAddr
}

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
	pc              *net.UDPConn
	initialized     bool
	streamType      StreamType
	writeTimeout    time.Duration
	readBuf         *multibuffer.MultiBuffer
	publishersMutex sync.RWMutex
	publishers      map[publisherAddr]*publisherData
	ringBuffer      *ringbuffer.RingBuffer

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

	err = pc.SetReadBuffer(serverConnUDPListenerKernelReadBufferSize)
	if err != nil {
		return nil, err
	}

	return &ServerUDPListener{
		pc:         pc,
		publishers: make(map[publisherAddr]*publisherData),
		done:       make(chan struct{}),
	}, nil
}

// Close closes the listener.
func (s *ServerUDPListener) Close() {
	s.pc.Close()

	if s.initialized {
		s.ringBuffer.Close()
		<-s.done
	}
}

func (s *ServerUDPListener) initialize(conf ServerConf, streamType StreamType) {
	if s.initialized {
		return
	}

	s.initialized = true
	s.streamType = streamType
	s.writeTimeout = conf.WriteTimeout
	s.readBuf = multibuffer.New(conf.ReadBufferCount, serverConnUDPListenerReadBufferSize)
	s.ringBuffer = ringbuffer.New(conf.ReadBufferCount)
	go s.run()
}

func (s *ServerUDPListener) run() {
	defer close(s.done)

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()

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
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()

		for {
			tmp, ok := s.ringBuffer.Pull()
			if !ok {
				return
			}
			pair := tmp.(bufAddrPair)

			s.pc.SetWriteDeadline(time.Now().Add(s.writeTimeout))
			s.pc.WriteTo(pair.buf, pair.addr)
		}
	}()

	wg.Wait()
}

func (s *ServerUDPListener) port() int {
	return s.pc.LocalAddr().(*net.UDPAddr).Port
}

func (s *ServerUDPListener) write(buf []byte, addr *net.UDPAddr) {
	s.ringBuffer.Push(bufAddrPair{buf, addr})
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
