package gortsplib

import (
	"fmt"
	"sync"
	"time"

	"github.com/pion/rtcp"
	"github.com/pion/rtp"

	"github.com/aler9/gortsplib/pkg/liberrors"
	"github.com/aler9/gortsplib/pkg/rtcpsender"
)

type serverStreamTrack struct {
	firstPacketSent    bool
	lastSequenceNumber uint16
	lastSSRC           uint32
	lastTimeRTP        uint32
	lastTimeNTP        time.Time
	udpRTCPSender      *rtcpsender.RTCPSender
}

// ServerStream represents a data stream.
// This is in charge of
// - distributing the stream to each reader
// - allocating multicast listeners
// - gathering infos about the stream to generate SSRC and RTP-Info
type ServerStream struct {
	tracks Tracks

	mutex                   sync.RWMutex
	s                       *Server
	activeUnicastReaders    map[*ServerSession]struct{}
	readers                 map[*ServerSession]struct{}
	serverMulticastHandlers []*serverMulticastHandler
	ssTracks                []*serverStreamTrack
}

// NewServerStream allocates a ServerStream.
func NewServerStream(tracks Tracks) *ServerStream {
	tracks = tracks.clone()
	tracks.setControls()

	st := &ServerStream{
		tracks:               tracks,
		activeUnicastReaders: make(map[*ServerSession]struct{}),
		readers:              make(map[*ServerSession]struct{}),
	}

	st.ssTracks = make([]*serverStreamTrack, len(tracks))
	for i := range st.ssTracks {
		st.ssTracks[i] = &serverStreamTrack{}
	}

	return st
}

// Close closes a ServerStream.
func (st *ServerStream) Close() error {
	st.mutex.Lock()
	defer st.mutex.Unlock()

	for ss := range st.readers {
		ss.Close()
	}

	if st.serverMulticastHandlers != nil {
		for _, h := range st.serverMulticastHandlers {
			h.close()
		}
		st.serverMulticastHandlers = nil
	}

	st.readers = nil
	st.activeUnicastReaders = nil

	return nil
}

// Tracks returns the tracks of the stream.
func (st *ServerStream) Tracks() Tracks {
	return st.tracks
}

func (st *ServerStream) ssrc(trackID int) uint32 {
	st.mutex.Lock()
	defer st.mutex.Unlock()
	return st.ssTracks[trackID].lastSSRC
}

func (st *ServerStream) rtpInfo(trackID int, now time.Time) (uint16, uint32, bool) {
	st.mutex.Lock()
	defer st.mutex.Unlock()

	track := st.ssTracks[trackID]

	if !track.firstPacketSent {
		return 0, 0, false
	}

	// sequence number of the first packet of the stream
	seq := track.lastSequenceNumber + 1

	// RTP timestamp corresponding to the time value in
	// the Range response header.
	// remove a small quantity in order to avoid DTS > PTS
	cr := st.tracks[trackID].ClockRate()
	ts := uint32(uint64(track.lastTimeRTP) +
		uint64(now.Sub(track.lastTimeNTP).Seconds()*float64(cr)) -
		uint64(cr)/10)

	return seq, ts, true
}

func (st *ServerStream) readerAdd(
	ss *ServerSession,
	transport Transport,
	clientPorts *[2]int,
) error {
	st.mutex.Lock()
	defer st.mutex.Unlock()

	if st.readers == nil {
		return fmt.Errorf("stream is closed")
	}

	switch transport {
	case TransportUDP:
		// check if client ports are already in use by another reader
		for r := range st.readers {
			if *r.setuppedTransport == TransportUDP &&
				r.author.ip().Equal(ss.author.ip()) &&
				r.author.zone() == ss.author.zone() {
				for _, rt := range r.setuppedTracks {
					if rt.udpRTPReadPort == clientPorts[0] {
						return liberrors.ErrServerUDPPortsAlreadyInUse{Port: rt.udpRTPReadPort}
					}
				}
			}
		}

	case TransportUDPMulticast:
		// allocate multicast listeners
		if st.serverMulticastHandlers == nil {
			st.serverMulticastHandlers = make([]*serverMulticastHandler, len(st.tracks))

			for i := range st.tracks {
				h, err := newServerMulticastHandler(st.s)
				if err != nil {
					for _, h := range st.serverMulticastHandlers {
						if h != nil {
							h.close()
						}
					}
					st.serverMulticastHandlers = nil
					return err
				}

				st.serverMulticastHandlers[i] = h
			}
		}
	}

	if st.s == nil {
		st.s = ss.s

		for trackID, track := range st.ssTracks {
			cTrackID := trackID

			// always generate RTCP sender reports.
			// they're mandatory when transport protocol is UDP or UDP-multicast.
			// they're also needed when transport protocol is TCP and client is Nvidia Deepstream
			// since they're used to compute NTP timestamp of frames:
			// https://docs.nvidia.com/metropolis/deepstream/dev-guide/text/DS_NTP_Timestamp.html
			track.udpRTCPSender = rtcpsender.New(
				st.s.udpSenderReportPeriod,
				st.tracks[trackID].ClockRate(),
				func(pkt rtcp.Packet) {
					st.WritePacketRTCP(cTrackID, pkt)
				},
			)
		}
	}

	st.readers[ss] = struct{}{}

	return nil
}

func (st *ServerStream) readerRemove(ss *ServerSession) {
	st.mutex.Lock()
	defer st.mutex.Unlock()

	delete(st.readers, ss)

	if len(st.readers) == 0 && st.serverMulticastHandlers != nil {
		for _, l := range st.serverMulticastHandlers {
			l.rtpl.close()
			l.rtcpl.close()
		}
		st.serverMulticastHandlers = nil
	}
}

func (st *ServerStream) readerSetActive(ss *ServerSession) {
	st.mutex.Lock()
	defer st.mutex.Unlock()

	switch *ss.setuppedTransport {
	case TransportUDP, TransportTCP:
		st.activeUnicastReaders[ss] = struct{}{}

	default: // UDPMulticast
		for trackID, track := range ss.setuppedTracks {
			st.serverMulticastHandlers[trackID].rtcpl.addClient(
				ss.author.ip(), st.serverMulticastHandlers[trackID].rtcpl.port(), ss, track, false)
		}
	}
}

func (st *ServerStream) readerSetInactive(ss *ServerSession) {
	st.mutex.Lock()
	defer st.mutex.Unlock()

	switch *ss.setuppedTransport {
	case TransportUDP, TransportTCP:
		delete(st.activeUnicastReaders, ss)

	default: // UDPMulticast
		if st.serverMulticastHandlers != nil {
			for trackID := range ss.setuppedTracks {
				st.serverMulticastHandlers[trackID].rtcpl.removeClient(ss)
			}
		}
	}
}

// WritePacketRTP writes a RTP packet to all the readers of the stream.
func (st *ServerStream) WritePacketRTP(trackID int, pkt *rtp.Packet, ptsEqualsDTS bool) {
	byts := make([]byte, maxPacketSize)
	n, err := pkt.MarshalTo(byts)
	if err != nil {
		return
	}
	byts = byts[:n]

	st.mutex.RLock()
	defer st.mutex.RUnlock()

	track := st.ssTracks[trackID]
	now := time.Now()

	if !track.firstPacketSent || ptsEqualsDTS {
		track.firstPacketSent = true
		track.lastTimeRTP = pkt.Header.Timestamp
		track.lastTimeNTP = now
	}

	track.lastSequenceNumber = pkt.Header.SequenceNumber
	track.lastSSRC = pkt.Header.SSRC

	if track.udpRTCPSender != nil {
		track.udpRTCPSender.ProcessPacketRTP(now, pkt, ptsEqualsDTS)
	}

	// send unicast
	for r := range st.activeUnicastReaders {
		r.writePacketRTP(trackID, byts)
	}

	// send multicast
	if st.serverMulticastHandlers != nil {
		st.serverMulticastHandlers[trackID].writePacketRTP(byts)
	}
}

// WritePacketRTCP writes a RTCP packet to all the readers of the stream.
func (st *ServerStream) WritePacketRTCP(trackID int, pkt rtcp.Packet) {
	byts, err := pkt.Marshal()
	if err != nil {
		return
	}

	st.mutex.RLock()
	defer st.mutex.RUnlock()

	// send unicast
	for r := range st.activeUnicastReaders {
		r.writePacketRTCP(trackID, byts)
	}

	// send multicast
	if st.serverMulticastHandlers != nil {
		st.serverMulticastHandlers[trackID].writePacketRTCP(byts)
	}
}
