package gortsplib

import (
	"fmt"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pion/rtcp"
	"golang.org/x/net/ipv4"
)

type clientData struct {
	ss           *ServerSession
	track        *ServerSessionSetuppedTrack
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

	pc           *net.UDPConn
	listenIP     net.IP
	isRTP        bool
	writeTimeout time.Duration
	clientsMutex sync.RWMutex
	clients      map[clientAddr]*clientData

	readerDone chan struct{}
}

func newServerUDPListenerMulticastPair(s *Server) (*serverUDPListener, *serverUDPListener, error) {
	res := make(chan net.IP)
	select {
	case s.streamMulticastIP <- streamMulticastIPReq{res: res}:
	case <-s.ctx.Done():
		return nil, nil, fmt.Errorf("terminated")
	}
	ip := <-res

	rtpl, err := newServerUDPListener(s, true,
		ip.String()+":"+strconv.FormatInt(int64(s.MulticastRTPPort), 10), true)
	if err != nil {
		return nil, nil, err
	}

	rtcpl, err := newServerUDPListener(s, true,
		ip.String()+":"+strconv.FormatInt(int64(s.MulticastRTCPPort), 10), false)
	if err != nil {
		rtpl.close()
		return nil, nil, err
	}

	return rtpl, rtcpl, nil
}

func newServerUDPListener(
	s *Server,
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

		tmp, err := s.ListenPacket("udp", "224.0.0.0:"+port)
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
		tmp, err := s.ListenPacket("udp", address)
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
		s:            s,
		pc:           pc,
		listenIP:     listenIP,
		clients:      make(map[clientAddr]*clientData),
		isRTP:        isRTP,
		writeTimeout: s.WriteTimeout,
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

	var processFunc func(*clientData, []byte)
	if u.isRTP {
		processFunc = u.processRTP
	} else {
		processFunc = u.processRTCP
	}

	for {
		buf := make([]byte, maxPacketSize)
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

			processFunc(clientData, buf[:n])
		}()
	}
}

func (u *serverUDPListener) processRTP(clientData *clientData, payload []byte) {
	pkt := u.s.udpRTPPacketBuffer.next()
	err := pkt.Unmarshal(payload)
	if err != nil {
		if h, ok := clientData.ss.s.Handler.(ServerHandlerOnDecodeError); ok {
			h.OnDecodeError(&ServerHandlerOnDecodeErrorCtx{
				Session: clientData.ss,
				Error:   err,
			})
		}
		return
	}

	packets, missing := clientData.track.reorderer.Process(pkt)

	if missing != 0 {
		if h, ok := clientData.ss.s.Handler.(ServerHandlerOnDecodeError); ok {
			h.OnDecodeError(&ServerHandlerOnDecodeErrorCtx{
				Session: clientData.ss,
				Error:   fmt.Errorf("%d RTP packet(s) lost", missing),
			})
		}
	}

	for _, pkt := range packets {
		now := time.Now()
		atomic.StoreInt64(clientData.ss.udpLastFrameTime, now.Unix())

		out, err := clientData.track.cleaner.Process(pkt)
		if err != nil {
			if h, ok := clientData.ss.s.Handler.(ServerHandlerOnDecodeError); ok {
				h.OnDecodeError(&ServerHandlerOnDecodeErrorCtx{
					Session: clientData.ss,
					Error:   err,
				})
			}
			continue
		}

		out0 := out[0]

		clientData.track.udpRTCPReceiver.ProcessPacketRTP(now, pkt, out0.PTSEqualsDTS)

		if h, ok := clientData.ss.s.Handler.(ServerHandlerOnPacketRTP); ok {
			h.OnPacketRTP(&ServerHandlerOnPacketRTPCtx{
				Session:      clientData.ss,
				TrackID:      clientData.track.id,
				Packet:       out0.Packet,
				PTSEqualsDTS: out0.PTSEqualsDTS,
				H264NALUs:    out0.H264NALUs,
				H264PTS:      out0.H264PTS,
			})
		}
	}
}

func (u *serverUDPListener) processRTCP(clientData *clientData, payload []byte) {
	packets, err := rtcp.Unmarshal(payload)
	if err != nil {
		if h, ok := clientData.ss.s.Handler.(ServerHandlerOnDecodeError); ok {
			h.OnDecodeError(&ServerHandlerOnDecodeErrorCtx{
				Session: clientData.ss,
				Error:   err,
			})
		}
		return
	}

	if clientData.isPublishing {
		now := time.Now()
		atomic.StoreInt64(clientData.ss.udpLastFrameTime, now.Unix())

		for _, pkt := range packets {
			clientData.track.udpRTCPReceiver.ProcessPacketRTCP(now, pkt)
		}
	}

	for _, pkt := range packets {
		clientData.ss.onPacketRTCP(clientData.track.id, pkt)
	}
}

func (u *serverUDPListener) write(buf []byte, addr *net.UDPAddr) error {
	// no mutex is needed here since Write() has an internal lock.
	// https://github.com/golang/go/issues/27203#issuecomment-534386117

	u.pc.SetWriteDeadline(time.Now().Add(u.writeTimeout))
	_, err := u.pc.WriteTo(buf, addr)
	return err
}

func (u *serverUDPListener) addClient(ip net.IP, port int, ss *ServerSession,
	track *ServerSessionSetuppedTrack, isPublishing bool,
) {
	u.clientsMutex.Lock()
	defer u.clientsMutex.Unlock()

	var addr clientAddr
	addr.fill(ip, port)

	u.clients[addr] = &clientData{
		ss:           ss,
		track:        track,
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
