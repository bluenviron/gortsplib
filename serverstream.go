package gortsplib

import (
	"encoding/binary"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/aler9/gortsplib/pkg/liberrors"
	"github.com/aler9/gortsplib/pkg/ringbuffer"
)

type trackTypePayload struct {
	trackID int
	isRTP   bool
	payload []byte
}

type multicastHandler struct {
	rtpl        *serverUDPListener
	rtcpl       *serverUDPListener
	writeBuffer *ringbuffer.RingBuffer

	writerDone chan struct{}
}

func newMulticastHandler(s *Server) (*multicastHandler, error) {
	rtpl, rtcpl, err := newServerUDPListenerMulticastPair(s)
	if err != nil {
		return nil, err
	}

	h := &multicastHandler{
		rtpl:        rtpl,
		rtcpl:       rtcpl,
		writeBuffer: ringbuffer.New(uint64(s.ReadBufferCount)),
		writerDone:  make(chan struct{}),
	}

	go h.runWriter()

	return h, nil
}

func (h *multicastHandler) close() {
	h.rtpl.close()
	h.rtcpl.close()
	h.writeBuffer.Close()
	<-h.writerDone
}

func (h *multicastHandler) ip() net.IP {
	return h.rtpl.ip()
}

func (h *multicastHandler) runWriter() {
	defer close(h.writerDone)

	rtpAddr := &net.UDPAddr{
		IP:   h.rtpl.ip(),
		Port: h.rtpl.port(),
	}

	rtcpAddr := &net.UDPAddr{
		IP:   h.rtcpl.ip(),
		Port: h.rtcpl.port(),
	}

	for {
		tmp, ok := h.writeBuffer.Pull()
		if !ok {
			return
		}
		data := tmp.(trackTypePayload)

		if data.isRTP {
			h.rtpl.write(data.payload, rtpAddr)
		} else {
			h.rtcpl.write(data.payload, rtcpAddr)
		}
	}
}

func (h *multicastHandler) writeRTP(payload []byte) {
	h.writeBuffer.Push(trackTypePayload{
		isRTP:   true,
		payload: payload,
	})
}

func (h *multicastHandler) writeRTCP(payload []byte) {
	h.writeBuffer.Push(trackTypePayload{
		isRTP:   false,
		payload: payload,
	})
}

type trackInfo struct {
	lastSequenceNumber uint32
	lastTimeRTP        uint32
	lastTimeNTP        int64
	lastSSRC           uint32
}

// ServerStream represents a single stream.
// This is in charge of
// - distributing the stream to each reader
// - allocating multicast listeners
// - gathering infos about the stream to generate SSRC and RTP-Info
type ServerStream struct {
	s      *Server
	tracks Tracks

	mutex             sync.RWMutex
	readersUnicast    map[*ServerSession]struct{}
	readers           map[*ServerSession]struct{}
	multicastHandlers []*multicastHandler
	trackInfos        []*trackInfo
}

// NewServerStream allocates a ServerStream.
func NewServerStream(tracks Tracks) *ServerStream {
	st := &ServerStream{
		readersUnicast: make(map[*ServerSession]struct{}),
		readers:        make(map[*ServerSession]struct{}),
	}

	st.tracks = cloneAndClearTracks(tracks)

	st.trackInfos = make([]*trackInfo, len(tracks))
	for i := range st.trackInfos {
		st.trackInfos[i] = &trackInfo{}
	}

	return st
}

// Close closes a ServerStream.
func (st *ServerStream) Close() error {
	st.mutex.Lock()
	defer st.mutex.Unlock()

	if st.s != nil {
		select {
		case st.s.streamRemove <- st:
		case <-st.s.ctx.Done():
		}
	}

	for ss := range st.readers {
		ss.Close()
	}

	if st.multicastHandlers != nil {
		for _, h := range st.multicastHandlers {
			h.close()
		}
		st.multicastHandlers = nil
	}

	st.readers = nil
	st.readersUnicast = nil

	return nil
}

// Tracks returns the tracks of the stream.
func (st *ServerStream) Tracks() Tracks {
	return st.tracks
}

func (st *ServerStream) ssrc(trackID int) uint32 {
	return atomic.LoadUint32(&st.trackInfos[trackID].lastSSRC)
}

func (st *ServerStream) timestamp(trackID int) uint32 {
	lastTimeRTP := atomic.LoadUint32(&st.trackInfos[trackID].lastTimeRTP)
	lastTimeNTP := atomic.LoadInt64(&st.trackInfos[trackID].lastTimeNTP)
	clockRate, _ := st.tracks[trackID].ClockRate()

	if lastTimeRTP == 0 || lastTimeNTP == 0 {
		return 0
	}

	return uint32(uint64(lastTimeRTP) +
		uint64(time.Since(time.Unix(lastTimeNTP, 0)).Seconds()*float64(clockRate)))
}

func (st *ServerStream) lastSequenceNumber(trackID int) uint16 {
	return uint16(atomic.LoadUint32(&st.trackInfos[trackID].lastSequenceNumber))
}

func (st *ServerStream) readerAdd(
	ss *ServerSession,
	transport Transport,
	clientPorts *[2]int,
) error {
	st.mutex.Lock()
	defer st.mutex.Unlock()

	if st.s == nil {
		st.s = ss.s
		select {
		case st.s.streamAdd <- st:
		case <-st.s.ctx.Done():
		}
	}

	switch transport {
	case TransportUDP:
		// check whether client ports are already in use by another reader.
		for r := range st.readersUnicast {
			if *r.setuppedTransport == TransportUDP &&
				r.author.ip().Equal(ss.author.ip()) &&
				r.author.zone() == ss.author.zone() {
				for _, rt := range r.setuppedTracks {
					if rt.udpRTPPort == clientPorts[0] {
						return liberrors.ErrServerUDPPortsAlreadyInUse{Port: rt.udpRTPPort}
					}
				}
			}
		}

	case TransportUDPMulticast:
		// allocate multicast listeners
		if st.multicastHandlers == nil {
			st.multicastHandlers = make([]*multicastHandler, len(st.tracks))

			for i := range st.tracks {
				h, err := newMulticastHandler(st.s)
				if err != nil {
					for _, h := range st.multicastHandlers {
						if h != nil {
							h.close()
						}
					}
					st.multicastHandlers = nil
					return err
				}

				st.multicastHandlers[i] = h
			}
		}
	}

	st.readers[ss] = struct{}{}

	return nil
}

func (st *ServerStream) readerRemove(ss *ServerSession) {
	st.mutex.Lock()
	defer st.mutex.Unlock()

	delete(st.readers, ss)

	if len(st.readers) == 0 && st.multicastHandlers != nil {
		for _, l := range st.multicastHandlers {
			l.rtpl.close()
			l.rtcpl.close()
		}
		st.multicastHandlers = nil
	}
}

func (st *ServerStream) readerSetActive(ss *ServerSession) {
	st.mutex.Lock()
	defer st.mutex.Unlock()

	switch *ss.setuppedTransport {
	case TransportUDP, TransportTCP:
		st.readersUnicast[ss] = struct{}{}

	default: // UDPMulticast
		for trackID := range ss.setuppedTracks {
			st.multicastHandlers[trackID].rtcpl.addClient(
				ss.author.ip(), st.multicastHandlers[trackID].rtcpl.port(), ss, trackID, false)
		}
	}
}

func (st *ServerStream) readerSetInactive(ss *ServerSession) {
	st.mutex.Lock()
	defer st.mutex.Unlock()

	switch *ss.setuppedTransport {
	case TransportUDP, TransportTCP:
		delete(st.readersUnicast, ss)

	default: // UDPMulticast
		if st.multicastHandlers != nil {
			for trackID := range ss.setuppedTracks {
				st.multicastHandlers[trackID].rtcpl.removeClient(ss)
			}
		}
	}
}

// WritePacketRTP writes a RTP packet to all the readers of the stream.
func (st *ServerStream) WritePacketRTP(trackID int, payload []byte) {
	if len(payload) >= 8 {
		track := st.trackInfos[trackID]

		sequenceNumber := binary.BigEndian.Uint16(payload[2:4])
		atomic.StoreUint32(&track.lastSequenceNumber, uint32(sequenceNumber))

		timestamp := binary.BigEndian.Uint32(payload[4:8])
		atomic.StoreUint32(&track.lastTimeRTP, timestamp)
		atomic.StoreInt64(&track.lastTimeNTP, time.Now().Unix())

		ssrc := binary.BigEndian.Uint32(payload[8:12])
		atomic.StoreUint32(&track.lastSSRC, ssrc)
	}

	st.mutex.RLock()
	defer st.mutex.RUnlock()

	// send unicast
	for r := range st.readersUnicast {
		r.writePacketRTP(trackID, payload)
	}

	// send multicast
	if st.multicastHandlers != nil {
		st.multicastHandlers[trackID].writeRTP(payload)
	}
}

// WritePacketRTCP writes a RTCP packet to all the readers of the stream.
func (st *ServerStream) WritePacketRTCP(trackID int, payload []byte) {
	st.mutex.RLock()
	defer st.mutex.RUnlock()

	// send unicast
	for r := range st.readersUnicast {
		r.WritePacketRTCP(trackID, payload)
	}

	// send multicast
	if st.multicastHandlers != nil {
		st.multicastHandlers[trackID].writeRTCP(payload)
	}
}
