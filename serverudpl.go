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

type serverUDPListener struct {
	pc              *net.UDPConn
	streamType      StreamType
	writeTimeout    time.Duration
	readBuf         *multibuffer.MultiBuffer
	publishersMutex sync.RWMutex
	publishers      map[publisherAddr]*publisherData
	ringBuffer      *ringbuffer.RingBuffer

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
		pc:         pc,
		publishers: make(map[publisherAddr]*publisherData),
		done:       make(chan struct{}),
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
				atomic.StoreInt64(pubData.publisher.announcedTracks[pubData.trackID].udpLastFrameTime, now.Unix())
				pubData.publisher.announcedTracks[pubData.trackID].rtcpReceiver.ProcessFrame(now, s.streamType, buf[:n])
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

func (s *serverUDPListener) port() int {
	return s.pc.LocalAddr().(*net.UDPAddr).Port
}

func (s *serverUDPListener) write(buf []byte, addr *net.UDPAddr) {
	s.ringBuffer.Push(bufAddrPair{buf, addr})
}

func (s *serverUDPListener) addPublisher(ip net.IP, port int, trackID int, sc *ServerConn) {
	s.publishersMutex.Lock()
	defer s.publishersMutex.Unlock()

	var addr publisherAddr
	addr.fill(ip, port)

	s.publishers[addr] = &publisherData{
		publisher: sc,
		trackID:   trackID,
	}
}

func (s *serverUDPListener) removePublisher(ip net.IP, port int) {
	s.publishersMutex.Lock()
	defer s.publishersMutex.Unlock()

	var addr publisherAddr
	addr.fill(ip, port)

	delete(s.publishers, addr)
}
