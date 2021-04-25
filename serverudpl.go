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
	s *Server,
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

	u := &serverUDPListener{
		pc:      pc,
		clients: make(map[clientAddr]*clientData),
		done:    make(chan struct{}),
	}

	u.streamType = streamType
	u.writeTimeout = s.WriteTimeout
	u.readBuf = multibuffer.New(uint64(s.ReadBufferCount), uint64(s.ReadBufferSize))
	u.ringBuffer = ringbuffer.New(uint64(s.ReadBufferCount))

	go u.run()

	return u, nil
}

func (u *serverUDPListener) close() {
	u.pc.Close()
	u.ringBuffer.Close()
	<-u.done
}

func (u *serverUDPListener) run() {
	defer close(u.done)

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()

		for {
			buf := u.readBuf.Next()
			n, addr, err := u.pc.ReadFromUDP(buf)
			if err != nil {
				break
			}

			func() {
				u.clientsMutex.RLock()
				defer u.clientsMutex.RUnlock()

				var clientAddr clientAddr
				clientAddr.fill(addr.IP, addr.Port)
				clientData, ok := u.clients[clientAddr]
				if !ok {
					return
				}

				if clientData.isPublishing {
					now := time.Now()
					atomic.StoreInt64(clientData.sc.announcedTracks[clientData.trackID].udpLastFrameTime, now.Unix())
					clientData.sc.announcedTracks[clientData.trackID].rtcpReceiver.ProcessFrame(now, u.streamType, buf[:n])
				}

				clientData.sc.readHandlers.OnFrame(clientData.trackID, u.streamType, buf[:n])
			}()
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()

		for {
			tmp, ok := u.ringBuffer.Pull()
			if !ok {
				return
			}
			pair := tmp.(bufAddrPair)

			u.pc.SetWriteDeadline(time.Now().Add(u.writeTimeout))
			u.pc.WriteTo(pair.buf, pair.addr)
		}
	}()

	wg.Wait()
}

func (u *serverUDPListener) port() int {
	return u.pc.LocalAddr().(*net.UDPAddr).Port
}

func (u *serverUDPListener) write(buf []byte, addr *net.UDPAddr) {
	u.ringBuffer.Push(bufAddrPair{buf, addr})
}

func (u *serverUDPListener) addClient(ip net.IP, port int, sc *ServerConn, trackID int, isPublishing bool) {
	u.clientsMutex.Lock()
	defer u.clientsMutex.Unlock()

	var addr clientAddr
	addr.fill(ip, port)

	u.clients[addr] = &clientData{
		sc:           sc,
		trackID:      trackID,
		isPublishing: isPublishing,
	}
}

func (u *serverUDPListener) removeClient(ip net.IP, port int) {
	u.clientsMutex.Lock()
	defer u.clientsMutex.Unlock()

	var addr clientAddr
	addr.fill(ip, port)

	delete(u.clients, addr)
}
