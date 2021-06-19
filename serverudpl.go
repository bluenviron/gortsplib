package gortsplib

import (
	"context"
	"math/rand"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/net/ipv4"

	"github.com/aler9/gortsplib/pkg/multibuffer"
	"github.com/aler9/gortsplib/pkg/ringbuffer"
)

const (
	serverConnUDPListenerKernelReadBufferSize = 0x80000 // same as gstreamer's rtspsrc
)

var multicastIP = net.ParseIP("239.0.0.0")

type bufAddrPair struct {
	buf  []byte
	addr *net.UDPAddr
}

type clientData struct {
	ss           *ServerSession
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
	s *Server

	ctx          context.Context
	ctxCancel    func()
	wg           sync.WaitGroup
	pc           *net.UDPConn
	streamType   StreamType
	writeTimeout time.Duration
	readBuf      *multibuffer.MultiBuffer
	clientsMutex sync.RWMutex
	clients      map[clientAddr]*clientData
	ringBuffer   *ringbuffer.RingBuffer
}

func newServerUDPListenerMulticastPair(s *Server) (*serverUDPListener, *serverUDPListener) {
	// choose two consecutive ports in range 65535-10000
	// rtp must be even and rtcp odd
	for {
		rtpPort := (rand.Intn((65535-10000)/2) * 2) + 10000
		rtpListener, err := newServerUDPListener(s, true, multicastIP.String()+":"+strconv.FormatInt(int64(rtpPort), 10), StreamTypeRTP)
		if err != nil {
			continue
		}

		rtcpPort := rtpPort + 1
		rtcpListener, err := newServerUDPListener(s, true, multicastIP.String()+":"+strconv.FormatInt(int64(rtcpPort), 10), StreamTypeRTCP)
		if err != nil {
			rtpListener.close()
			continue
		}

		return rtpListener, rtcpListener
	}
}

func newServerUDPListener(
	s *Server,
	multicast bool,
	address string,
	streamType StreamType) (*serverUDPListener, error) {
	var pc *net.UDPConn
	if multicast {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}

		tmp, err := s.ListenPacket("udp", "224.0.0.0:"+port)
		if err != nil {
			return nil, err
		}

		p := ipv4.NewPacketConn(tmp)

		err = p.SetTTL(127)
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
		tmp, err := s.ListenPacket("udp", address)
		if err != nil {
			return nil, err
		}
		pc = tmp.(*net.UDPConn)
	}

	err := pc.SetReadBuffer(serverConnUDPListenerKernelReadBufferSize)
	if err != nil {
		return nil, err
	}

	ctx, ctxCancel := context.WithCancel(context.Background())

	u := &serverUDPListener{
		s:         s,
		ctx:       ctx,
		ctxCancel: ctxCancel,
		pc:        pc,
		clients:   make(map[clientAddr]*clientData),
	}

	u.streamType = streamType
	u.writeTimeout = s.WriteTimeout
	u.readBuf = multibuffer.New(uint64(s.ReadBufferCount), uint64(s.ReadBufferSize))
	u.ringBuffer = ringbuffer.New(uint64(s.ReadBufferCount))

	u.wg.Add(1)
	go u.run()

	return u, nil
}

func (u *serverUDPListener) close() {
	u.ctxCancel()
	u.wg.Wait()
}

func (u *serverUDPListener) run() {
	defer u.wg.Done()

	u.wg.Add(1)
	go func() {
		defer u.wg.Done()

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
					atomic.StoreInt64(clientData.ss.udpLastFrameTime, now.Unix())
					clientData.ss.announcedTracks[clientData.trackID].rtcpReceiver.ProcessFrame(now, u.streamType, buf[:n])
				}

				if h, ok := u.s.Handler.(ServerHandlerOnFrame); ok {
					h.OnFrame(&ServerHandlerOnFrameCtx{
						Session:    clientData.ss,
						TrackID:    clientData.trackID,
						StreamType: u.streamType,
						Payload:    buf[:n],
					})
				}
			}()
		}
	}()

	u.wg.Add(1)
	go func() {
		defer u.wg.Done()

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

	<-u.ctx.Done()

	u.pc.Close()
	u.ringBuffer.Close()
}

func (u *serverUDPListener) port() int {
	return u.pc.LocalAddr().(*net.UDPAddr).Port
}

func (u *serverUDPListener) write(buf []byte, addr *net.UDPAddr) {
	u.ringBuffer.Push(bufAddrPair{buf, addr})
}

func (u *serverUDPListener) addClient(ip net.IP, port int, ss *ServerSession, trackID int, isPublishing bool) {
	u.clientsMutex.Lock()
	defer u.clientsMutex.Unlock()

	var addr clientAddr
	addr.fill(ip, port)

	u.clients[addr] = &clientData{
		ss:           ss,
		trackID:      trackID,
		isPublishing: isPublishing,
	}
}

func (u *serverUDPListener) removeClient(ss *ServerSession) {
	u.clientsMutex.Lock()
	defer u.clientsMutex.Unlock()

	for addr, data := range u.clients {
		if data.ss == ss {
			delete(u.clients, addr)
		}
	}
}
