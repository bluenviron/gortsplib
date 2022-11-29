package gortsplib

import (
	"fmt"
	"sync"
	"time"

	"github.com/pion/rtcp"
	"github.com/pion/rtp"

	"github.com/aler9/gortsplib/pkg/headers"
	"github.com/aler9/gortsplib/pkg/liberrors"
	"github.com/aler9/gortsplib/pkg/rtcpsender"
	"github.com/aler9/gortsplib/pkg/track"
)

type serverStreamMediaTrack struct {
	track      track.Track
	rtcpSender *rtcpsender.RTCPSender
}

type serverStreamMedia struct {
	tracks           map[uint8]*serverStreamMediaTrack
	multicastHandler *serverMulticastHandler
}

// ServerStream represents a data stream.
// This is in charge of
// - distributing the stream to each reader
// - allocating multicast listeners
// - gathering infos about the stream in order to generate SSRC and RTP-Info
type ServerStream struct {
	medias Medias

	mutex                sync.RWMutex
	s                    *Server
	activeUnicastReaders map[*ServerSession]struct{}
	readers              map[*ServerSession]struct{}
	streamMedias         []*serverStreamMedia
	closed               bool
}

// NewServerStream allocates a ServerStream.
func NewServerStream(medias Medias) *ServerStream {
	medias = medias.clone()
	medias.setControls()

	st := &ServerStream{
		medias:               medias,
		activeUnicastReaders: make(map[*ServerSession]struct{}),
		readers:              make(map[*ServerSession]struct{}),
	}

	st.streamMedias = make([]*serverStreamMedia, len(medias))
	for mediaID, media := range medias {
		ssm := &serverStreamMedia{}

		ssm.tracks = make(map[uint8]*serverStreamMediaTrack)
		for _, trak := range media.Tracks {
			tr := &serverStreamMediaTrack{
				track: trak,
			}

			ci := mediaID
			tr.rtcpSender = rtcpsender.New(
				trak.ClockRate(),
				func(pkt rtcp.Packet) {
					st.WritePacketRTCP(ci, pkt)
				},
			)

			ssm.tracks[trak.PayloadType()] = tr
		}

		st.streamMedias[mediaID] = ssm
	}

	return st
}

func (st *ServerStream) initializeServerDependentPart() {
	if !st.s.DisableRTCPSenderReports {
		for _, ssm := range st.streamMedias {
			for _, tr := range ssm.tracks {
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

	for _, ssm := range st.streamMedias {
		for _, tr := range ssm.tracks {
			if tr.rtcpSender != nil {
				tr.rtcpSender.Close()
			}
		}

		if ssm.multicastHandler != nil {
			ssm.multicastHandler.close()
		}
	}

	return nil
}

// Medias returns the medias of the stream.
func (st *ServerStream) Medias() Medias {
	return st.medias
}

func (st *ServerStream) lastSSRC(mediaID int) (uint32, bool) {
	st.mutex.Lock()
	defer st.mutex.Unlock()

	media := st.streamMedias[mediaID]

	// since lastSSRC() is used to fill SSRC inside the Transport header,
	// if there are multiple tracks inside a single media stream,
	// do not return anything, since Transport headers don't support multiple SSRCs.
	if len(media.tracks) > 1 {
		return 0, false
	}

	var firstKey uint8
	for key := range media.tracks {
		firstKey = key
		break
	}

	return media.tracks[firstKey].rtcpSender.LastSSRC()
}

func (st *ServerStream) rtpInfoEntry(mediaID int, now time.Time) *headers.RTPInfoEntry {
	st.mutex.Lock()
	defer st.mutex.Unlock()

	media := st.streamMedias[mediaID]

	// if there are multiple tracks inside a single media stream,
	// do not generate a RTP-Info entry, since RTP-Info doesn't support
	// multiple sequence numbers / timestamps.
	if len(media.tracks) > 1 {
		return nil
	}

	var firstKey uint8
	for key := range media.tracks {
		firstKey = key
		break
	}

	track := media.tracks[firstKey]

	lastSeqNum, lastTimeRTP, lastTimeNTP, ok := track.rtcpSender.LastPacketData()
	if !ok {
		return nil
	}

	clockRate := track.track.ClockRate()
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
			if media.multicastHandler == nil {
				mh, err := newServerMulticastHandler(st.s)
				if err != nil {
					return err
				}
				media.multicastHandler = mh
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
			if media.multicastHandler != nil {
				media.multicastHandler.close()
				media.multicastHandler = nil
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
		for mediaID, media := range ss.setuppedMedias {
			st.streamMedias[mediaID].multicastHandler.rtcpl.addClient(
				ss.author.ip(), st.streamMedias[mediaID].multicastHandler.rtcpl.port(), ss, media, false)
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
		for mediaID := range ss.setuppedMedias {
			st.streamMedias[mediaID].multicastHandler.rtcpl.removeClient(ss)
		}
	} else {
		delete(st.activeUnicastReaders, ss)
	}
}

// WritePacketRTP writes a RTP packet to all the readers of the stream.
func (st *ServerStream) WritePacketRTP(mediaID int, pkt *rtp.Packet) {
	st.WritePacketRTPWithNTP(mediaID, pkt, time.Now())
}

// WritePacketRTPWithNTP writes a RTP packet to all the readers of the stream.
// ntp is the absolute time of the packet, and is needed to generate RTCP sender reports
// that allows the receiver to reconstruct the absolute time of the packet.
func (st *ServerStream) WritePacketRTPWithNTP(mediaID int, pkt *rtp.Packet, ntp time.Time) {
	byts := make([]byte, maxPacketSize)
	n, err := pkt.MarshalTo(byts)
	if err != nil {
		return
	}
	byts = byts[:n]

	st.mutex.RLock()
	defer st.mutex.RUnlock()

	if st.closed {
		return
	}

	media := st.streamMedias[mediaID]

	track, ok := media.tracks[pkt.PayloadType]
	if !ok {
		return
	}

	track.rtcpSender.ProcessPacket(pkt, ntp, track.track.PTSEqualsDTS(pkt))

	// send unicast
	for r := range st.activeUnicastReaders {
		r.writePacketRTP(mediaID, byts)
	}

	// send multicast
	if media.multicastHandler != nil {
		media.multicastHandler.writePacketRTP(byts)
	}
}

// WritePacketRTCP writes a RTCP packet to all the readers of the stream.
func (st *ServerStream) WritePacketRTCP(mediaID int, pkt rtcp.Packet) {
	byts, err := pkt.Marshal()
	if err != nil {
		return
	}

	st.mutex.RLock()
	defer st.mutex.RUnlock()

	if st.closed {
		return
	}

	media := st.streamMedias[mediaID]

	// send unicast
	for r := range st.activeUnicastReaders {
		r.writePacketRTCP(mediaID, byts)
	}

	// send multicast
	if media.multicastHandler != nil {
		media.multicastHandler.writePacketRTCP(byts)
	}
}
