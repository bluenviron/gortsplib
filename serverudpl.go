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
)

type bufAddrPair struct {
	buf  []byte
	addr *net.UDPAddr
}

type clientData struct {
	sc           *ServerConn
	trackID      int
	isPublishing bool
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

type serverUDPListener struct {
	pc           *net.UDPConn
	streamType   StreamType
	writeTimeout time.Duration
	readBuf      *multibuffer.MultiBuffer
	clientsMutex sync.RWMutex
	clients      map[clientAddr]*clientData
	ringBuffer   *ringbuffer.RingBuffer

	// out
	done chan struct{}
}

func newServerUDPListener(
	conf ServerConf,
	address string,
	streamType StreamType) (*serverUDPListener, error) {

	tmp, err := net.ListenPacket("udp", address)
	if err != nil {
		return nil, err
	}
	pc := tmp.(*net.UDPConn)

	err = pc.SetReadBuffer(serverConnUDPListenerKernelReadBufferSize)
	if err != nil {
		return nil, err
	}

	s := &serverUDPListener{
		pc:      pc,
		clients: make(map[clientAddr]*clientData),
		done:    make(chan struct{}),
	}

	s.streamType = streamType
	s.writeTimeout = conf.WriteTimeout
	s.readBuf = multibuffer.New(uint64(conf.ReadBufferCount), uint64(conf.ReadBufferSize))
	s.ringBuffer = ringbuffer.New(uint64(conf.ReadBufferCount))

	go s.run()

	return s, nil
}

func (s *serverUDPListener) close() {
	s.pc.Close()
	s.ringBuffer.Close()
	<-s.done
}

func (s *serverUDPListener) run() {
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
				s.clientsMutex.RLock()
				defer s.clientsMutex.RUnlock()

				var clientAddr clientAddr
				clientAddr.fill(addr.IP, addr.Port)
				clientData, ok := s.clients[clientAddr]
				if !ok {
					return
				}

				if clientData.isPublishing {
					now := time.Now()
					atomic.StoreInt64(clientData.sc.announcedTracks[clientData.trackID].udpLastFrameTime, now.Unix())
					clientData.sc.announcedTracks[clientData.trackID].rtcpReceiver.ProcessFrame(now, s.streamType, buf[:n])
				}

				clientData.sc.readHandlers.OnFrame(clientData.trackID, s.streamType, buf[:n])
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

func (s *serverUDPListener) port() int {
	return s.pc.LocalAddr().(*net.UDPAddr).Port
}

func (s *serverUDPListener) write(buf []byte, addr *net.UDPAddr) {
	s.ringBuffer.Push(bufAddrPair{buf, addr})
}

func (s *serverUDPListener) addClient(ip net.IP, port int, sc *ServerConn, trackID int, isPublishing bool) {
	s.clientsMutex.Lock()
	defer s.clientsMutex.Unlock()

	var addr clientAddr
	addr.fill(ip, port)

	s.clients[addr] = &clientData{
		sc:           sc,
		trackID:      trackID,
		isPublishing: isPublishing,
	}
}

func (s *serverUDPListener) removeClient(ip net.IP, port int) {
	s.clientsMutex.Lock()
	defer s.clientsMutex.Unlock()

	var addr clientAddr
	addr.fill(ip, port)

	delete(s.clients, addr)
}
