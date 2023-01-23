package gortsplib

import (
	"fmt"
	"sync"
	"time"

	"github.com/pion/rtcp"
	"github.com/pion/rtp"

	"github.com/aler9/gortsplib/v2/pkg/headers"
	"github.com/aler9/gortsplib/v2/pkg/liberrors"
	"github.com/aler9/gortsplib/v2/pkg/media"
)

// ServerStream represents a data stream.
// This is in charge of
// - distributing the stream to each reader
// - allocating multicast listeners
// - gathering infos about the stream in order to generate SSRC and RTP-Info
type ServerStream struct {
	medias media.Medias

	mutex                sync.RWMutex
	s                    *Server
	activeUnicastReaders map[*ServerSession]struct{}
	readers              map[*ServerSession]struct{}
	streamMedias         map[*media.Media]*serverStreamMedia
	closed               bool
}

// NewServerStream allocates a ServerStream.
func NewServerStream(medias media.Medias) *ServerStream {
	st := &ServerStream{
		medias:               medias,
		activeUnicastReaders: make(map[*ServerSession]struct{}),
		readers:              make(map[*ServerSession]struct{}),
	}

	st.streamMedias = make(map[*media.Media]*serverStreamMedia, len(medias))
	for _, medi := range medias {
		st.streamMedias[medi] = newServerStreamMedia(st, medi)
	}

	return st
}

func (st *ServerStream) initializeServerDependentPart() {
	if !st.s.DisableRTCPSenderReports {
		for _, ssm := range st.streamMedias {
			for _, tr := range ssm.formats {
				tr.rtcpSender.Start(st.s.senderReportPeriod)
			}
		}
	}
}

// Close closes a ServerStream.
func (st *ServerStream) Close() error {
	st.mutex.Lock()
	st.closed = true
	st.mutex.Unlock()

	for ss := range st.readers {
		ss.Close()
	}

	for _, sm := range st.streamMedias {
		sm.close()
	}

	return nil
}

// Medias returns the medias of the stream.
func (st *ServerStream) Medias() media.Medias {
	return st.medias
}

func (st *ServerStream) lastSSRC(medi *media.Media) (uint32, bool) {
	st.mutex.Lock()
	defer st.mutex.Unlock()

	sm := st.streamMedias[medi]

	// since lastSSRC() is used to fill SSRC inside the Transport header,
	// if there are multiple formats inside a single media stream,
	// do not return anything, since Transport headers don't support multiple SSRCs.
	if len(sm.formats) > 1 {
		return 0, false
	}

	var firstKey uint8
	for key := range sm.formats {
		firstKey = key
		break
	}

	return sm.formats[firstKey].rtcpSender.LastSSRC()
}

func (st *ServerStream) rtpInfoEntry(medi *media.Media, now time.Time) *headers.RTPInfoEntry {
	st.mutex.Lock()
	defer st.mutex.Unlock()

	sm := st.streamMedias[medi]

	// if there are multiple formats inside a single media stream,
	// do not generate a RTP-Info entry, since RTP-Info doesn't support
	// multiple sequence numbers / timestamps.
	if len(sm.formats) > 1 {
		return nil
	}

	var firstKey uint8
	for key := range sm.formats {
		firstKey = key
		break
	}

	format := sm.formats[firstKey]

	lastSeqNum, lastTimeRTP, lastTimeNTP, ok := format.rtcpSender.LastPacketData()
	if !ok {
		return nil
	}

	clockRate := format.format.ClockRate()
	if clockRate == 0 {
		return nil
	}

	// sequence number of the first packet of the stream
	seqNum := lastSeqNum + 1

	// RTP timestamp corresponding to the time value in
	// the Range response header.
	// remove a small quantity in order to avoid DTS > PTS
	ts := uint32(uint64(lastTimeRTP) +
		uint64(now.Sub(lastTimeNTP).Seconds()*float64(clockRate)) -
		uint64(clockRate)/10)

	return &headers.RTPInfoEntry{
		SequenceNumber: &seqNum,
		Timestamp:      &ts,
	}
}

func (st *ServerStream) readerAdd(
	ss *ServerSession,
	transport Transport,
	clientPorts *[2]int,
) error {
	st.mutex.Lock()
	defer st.mutex.Unlock()

	if st.closed {
		return fmt.Errorf("stream is closed")
	}

	if st.s == nil {
		st.s = ss.s
		st.initializeServerDependentPart()
	}

	switch transport {
	case TransportUDP:
		// check whether UDP ports and IP are already assigned to another reader
		for r := range st.readers {
			if *r.setuppedTransport == TransportUDP &&
				r.author.ip().Equal(ss.author.ip()) &&
				r.author.zone() == ss.author.zone() {
				for _, rt := range r.setuppedMedias {
					if rt.udpRTPReadPort == clientPorts[0] {
						return liberrors.ErrServerUDPPortsAlreadyInUse{Port: rt.udpRTPReadPort}
					}
				}
			}
		}

	case TransportUDPMulticast:
		// allocate multicast listeners
		for _, media := range st.streamMedias {
			err := media.allocateMulticastHandler(st.s)
			if err != nil {
				return err
			}
		}
	}

	st.readers[ss] = struct{}{}

	return nil
}

func (st *ServerStream) readerRemove(ss *ServerSession) {
	st.mutex.Lock()
	defer st.mutex.Unlock()

	if st.closed {
		return
	}

	delete(st.readers, ss)

	if len(st.readers) == 0 {
		for _, media := range st.streamMedias {
			if media.multicastWriter != nil {
				media.multicastWriter.close()
				media.multicastWriter = nil
			}
		}
	}
}

func (st *ServerStream) readerSetActive(ss *ServerSession) {
	st.mutex.Lock()
	defer st.mutex.Unlock()

	if st.closed {
		return
	}

	if *ss.setuppedTransport == TransportUDPMulticast {
		for medi, sm := range ss.setuppedMedias {
			streamMedia := st.streamMedias[medi]
			streamMedia.multicastWriter.rtcpl.addClient(
				ss.author.ip(), streamMedia.multicastWriter.rtcpl.port(), sm)
		}
	} else {
		st.activeUnicastReaders[ss] = struct{}{}
	}
}

func (st *ServerStream) readerSetInactive(ss *ServerSession) {
	st.mutex.Lock()
	defer st.mutex.Unlock()

	if st.closed {
		return
	}

	if *ss.setuppedTransport == TransportUDPMulticast {
		for medi, sm := range ss.setuppedMedias {
			streamMedia := st.streamMedias[medi]
			streamMedia.multicastWriter.rtcpl.removeClient(sm)
		}
	} else {
		delete(st.activeUnicastReaders, ss)
	}
}

// WritePacketRTP writes a RTP packet to all the readers of the stream.
func (st *ServerStream) WritePacketRTP(medi *media.Media, pkt *rtp.Packet) {
	st.WritePacketRTPWithNTP(medi, pkt, time.Now())
}

// WritePacketRTPWithNTP writes a RTP packet to all the readers of the stream.
// ntp is the absolute time of the packet, and is needed to generate RTCP sender reports
// that allows the receiver to reconstruct the absolute time of the packet.
func (st *ServerStream) WritePacketRTPWithNTP(medi *media.Media, pkt *rtp.Packet, ntp time.Time) {
	st.mutex.RLock()
	defer st.mutex.RUnlock()

	if st.closed {
		return
	}

	sm := st.streamMedias[medi]
	sm.WritePacketRTPWithNTP(st, pkt, ntp)
}

// WritePacketRTCP writes a RTCP packet to all the readers of the stream.
func (st *ServerStream) WritePacketRTCP(medi *media.Media, pkt rtcp.Packet) {
	st.mutex.RLock()
	defer st.mutex.RUnlock()

	if st.closed {
		return
	}

	sm := st.streamMedias[medi]
	sm.writePacketRTCP(st, pkt)
}
