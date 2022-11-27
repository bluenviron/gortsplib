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
	lastSequenceNumber uint16
	lastSSRC           uint32
	lastTimeFilled     bool
	lastTimeRTP        uint32
	lastTimeNTP        time.Time
	rtcpSender         *rtcpsender.RTCPSender
	multicastHandler   *serverMulticastHandler
}

// ServerStream represents a data stream.
// This is in charge of
// - distributing the stream to each reader
// - allocating multicast listeners
// - gathering infos about the stream in order to generate SSRC and RTP-Info
type ServerStream struct {
	tracks Tracks

	mutex                sync.RWMutex
	s                    *Server
	activeUnicastReaders map[*ServerSession]struct{}
	readers              map[*ServerSession]struct{}
	streamTracks         []*serverStreamTrack
	closed               bool
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

	st.streamTracks = make([]*serverStreamTrack, len(tracks))
	for i := range st.streamTracks {
		st.streamTracks[i] = &serverStreamTrack{}
	}

	return st
}

// Close closes a ServerStream.
func (st *ServerStream) Close() error {
	st.mutex.Lock()
	st.closed = true
	st.mutex.Unlock()

	for ss := range st.readers {
		ss.Close()
	}

	for _, track := range st.streamTracks {
		if track.rtcpSender != nil {
			track.rtcpSender.Close()
		}
		if track.multicastHandler != nil {
			track.multicastHandler.close()
		}
	}

	return nil
}

// Tracks returns the tracks of the stream.
func (st *ServerStream) Tracks() Tracks {
	return st.tracks
}

func (st *ServerStream) ssrc(trackID int) uint32 {
	st.mutex.Lock()
	defer st.mutex.Unlock()
	return st.streamTracks[trackID].lastSSRC
}

func (st *ServerStream) rtpInfo(trackID int, now time.Time) (uint16, uint32, bool) {
	st.mutex.Lock()
	defer st.mutex.Unlock()

	track := st.streamTracks[trackID]

	if !track.lastTimeFilled {
		return 0, 0, false
	}

	clockRate := st.tracks[trackID].ClockRate()
	if clockRate == 0 {
		return 0, 0, false
	}

	// sequence number of the first packet of the stream
	seq := track.lastSequenceNumber + 1

	// RTP timestamp corresponding to the time value in
	// the Range response header.
	// remove a small quantity in order to avoid DTS > PTS
	ts := uint32(uint64(track.lastTimeRTP) +
		uint64(now.Sub(track.lastTimeNTP).Seconds()*float64(clockRate)) -
		uint64(clockRate)/10)

	return seq, ts, true
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

		if !st.s.DisableRTCPSenderReports {
			for trackID, track := range st.streamTracks {
				cTrackID := trackID
				track.rtcpSender = rtcpsender.New(
					st.s.udpSenderReportPeriod,
					st.tracks[trackID].ClockRate(),
					func(pkt rtcp.Packet) {
						st.WritePacketRTCP(cTrackID, pkt)
					},
				)
			}
		}
	}

	switch transport {
	case TransportUDP:
		// check whether UDP ports and IP are already assigned to another reader
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
		for _, track := range st.streamTracks {
			if track.multicastHandler == nil {
				mh, err := newServerMulticastHandler(st.s)
				if err != nil {
					return err
				}
				track.multicastHandler = mh
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
		for _, track := range st.streamTracks {
			if track.multicastHandler != nil {
				track.multicastHandler.close()
				track.multicastHandler = nil
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
		for trackID, track := range ss.setuppedTracks {
			st.streamTracks[trackID].multicastHandler.rtcpl.addClient(
				ss.author.ip(), st.streamTracks[trackID].multicastHandler.rtcpl.port(), ss, track, false)
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
		for trackID := range ss.setuppedTracks {
			st.streamTracks[trackID].multicastHandler.rtcpl.removeClient(ss)
		}
	} else {
		delete(st.activeUnicastReaders, ss)
	}
}

// WritePacketRTP writes a RTP packet to all the readers of the stream.
func (st *ServerStream) WritePacketRTP(trackID int, pkt *rtp.Packet) {
	st.WritePacketRTPWithNTP(trackID, pkt, time.Now())
}

// WritePacketRTPWithNTP writes a RTP packet to all the readers of the stream.
// ntp is the absolute time of the packet, and is needed to generate RTCP sender reports
// that allows the receiver to reconstruct the absolute time of the packet.
func (st *ServerStream) WritePacketRTPWithNTP(trackID int, pkt *rtp.Packet, ntp time.Time) {
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

	track := st.streamTracks[trackID]
	ptsEqualsDTS := ptsEqualsDTS(st.tracks[trackID], pkt)

	if ptsEqualsDTS {
		track.lastTimeFilled = true
		track.lastTimeRTP = pkt.Header.Timestamp
		track.lastTimeNTP = ntp
	}

	track.lastSequenceNumber = pkt.Header.SequenceNumber
	track.lastSSRC = pkt.Header.SSRC

	if track.rtcpSender != nil {
		track.rtcpSender.ProcessPacketRTP(ntp, pkt, ptsEqualsDTS)
	}

	// send unicast
	for r := range st.activeUnicastReaders {
		r.writePacketRTP(trackID, byts)
	}

	// send multicast
	if track.multicastHandler != nil {
		track.multicastHandler.writePacketRTP(byts)
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

	if st.closed {
		return
	}

	// send unicast
	for r := range st.activeUnicastReaders {
		r.writePacketRTCP(trackID, byts)
	}

	// send multicast
	track := st.streamTracks[trackID]
	if track.multicastHandler != nil {
		track.multicastHandler.writePacketRTCP(byts)
	}
}
