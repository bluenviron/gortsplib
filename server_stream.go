package gortsplib

import (
	"crypto/rand"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/pion/rtcp"
	"github.com/pion/rtp"

	"github.com/bluenviron/gortsplib/v5/pkg/description"
	"github.com/bluenviron/gortsplib/v5/pkg/liberrors"
)

func serverStreamExtractExistingSSRCs(medias map[*description.Media]*serverStreamMedia) []uint32 {
	n := 0
	for _, media := range medias {
		for range media.formats {
			n++
		}
	}

	if n == 0 {
		return nil
	}

	ret := make([]uint32, n)
	n = 0

	for _, media := range medias {
		for _, forma := range media.formats {
			ret[n] = forma.localSSRC
			n++
		}
	}

	return ret
}

// StreamMediaMulticastParams used to request specific Multicast configuration for each media in a stream
type StreamMediaMulticastParams struct {
	IP       net.IP
	RTPPort  int
	RTCPPort int
}

// ServerStream represents a media stream.
// This is in charge of
// - storing stream description and statistics
// - distributing the stream to each reader
// - allocating multicast listeners
type ServerStream struct {
	// Parent server.
	Server *Server

	// Stream description.
	Desc *description.Session

	// (optional) Stream-specific Multicast settings.
	MulticastParams map[*description.Media]StreamMediaMulticastParams

	mutex                sync.RWMutex
	readers              map[*ServerSession]struct{}
	multicastReaderCount int
	activeUnicastReaders map[*ServerSession]struct{}
	medias               map[*description.Media]*serverStreamMedia
	closed               bool
}

// Initialize initializes a ServerStream.
func (st *ServerStream) Initialize() error {
	if st.Server == nil || st.Server.sessions == nil {
		return fmt.Errorf("server not present or not initialized")
	}

	st.readers = make(map[*ServerSession]struct{})
	st.activeUnicastReaders = make(map[*ServerSession]struct{})

	st.medias = make(map[*description.Media]*serverStreamMedia, len(st.Desc.Medias))

	for i, medi := range st.Desc.Medias {
		localSSRCs, err := generateLocalSSRCs(
			serverStreamExtractExistingSSRCs(st.medias),
			medi.Formats,
		)
		if err != nil {
			return err
		}

		var srtpOutCtx *wrappedSRTPContext

		if st.Server.TLSConfig != nil {
			srtpOutKey := make([]byte, srtpKeyLength)
			_, err = rand.Read(srtpOutKey)
			if err != nil {
				return err
			}

			srtpOutCtx = &wrappedSRTPContext{
				key:   srtpOutKey,
				ssrcs: ssrcsMapToList(localSSRCs),
			}
			err = srtpOutCtx.initialize()
			if err != nil {
				return err
			}
		}

		sm := &serverStreamMedia{
			st:         st,
			media:      medi,
			trackID:    i,
			localSSRCs: localSSRCs,
			srtpOutCtx: srtpOutCtx,
		}
		sm.initialize()

		st.medias[medi] = sm
	}

	return nil
}

// Close closes a ServerStream.
func (st *ServerStream) Close() {
	st.mutex.Lock()

	st.closed = true

	for ss := range st.readers {
		st.readerSetInactiveUnsafe(ss)
		st.readerRemoveUnsafe(ss)
		ss.Close()
	}

	st.mutex.Unlock()
}

// Stats returns stream statistics.
func (st *ServerStream) Stats() *ServerStreamStats {
	mediaStats := func() map[*description.Media]ServerStreamStatsMedia {
		ret := make(map[*description.Media]ServerStreamStatsMedia, len(st.medias))
		for med, sm := range st.medias {
			ret[med] = sm.stats()
		}
		return ret
	}()

	return &ServerStreamStats{
		OutboundBytes: func() uint64 {
			v := uint64(0)
			for _, ms := range mediaStats {
				v += ms.OutboundBytes
			}
			return v
		}(),
		OutboundRTPPackets: func() uint64 {
			v := uint64(0)
			for _, ms := range mediaStats {
				for _, f := range ms.Formats {
					v += f.OutboundRTPPackets
				}
			}
			return v
		}(),
		OutboundRTCPPackets: func() uint64 {
			v := uint64(0)
			for _, ms := range mediaStats {
				v += ms.OutboundRTCPPackets
			}
			return v
		}(),
		Medias: mediaStats,
		// deprecated
		BytesSent: func() uint64 {
			v := uint64(0)
			for _, ms := range mediaStats {
				v += ms.OutboundBytes
			}
			return v
		}(),
		RTPPacketsSent: func() uint64 {
			v := uint64(0)
			for _, ms := range mediaStats {
				for _, f := range ms.Formats {
					v += f.OutboundRTPPackets
				}
			}
			return v
		}(),
		RTCPPacketsSent: func() uint64 {
			v := uint64(0)
			for _, ms := range mediaStats {
				v += ms.OutboundRTCPPackets
			}
			return v
		}(),
	}
}

func (st *ServerStream) readerAdd(
	ss *ServerSession,
	clientPorts *[2]int,
	protocol Protocol,
) error {
	st.mutex.Lock()
	defer st.mutex.Unlock()

	if st.closed {
		return liberrors.ErrServerStreamClosed{}
	}

	switch protocol {
	case ProtocolUDP:
		// check whether UDP ports and IP are already assigned to another reader
		for r := range st.readers {
			if protocol == ProtocolUDP &&
				r.author.ip().Equal(ss.author.ip()) &&
				r.author.zone() == ss.author.zone() {
				for _, rt := range r.setuppedMedias {
					if rt.udpRTPReadPort == clientPorts[0] {
						return liberrors.ErrServerUDPPortsAlreadyInUse{Port: rt.udpRTPReadPort}
					}
				}
			}
		}

	case ProtocolUDPMulticast:
		if st.multicastReaderCount == 0 {
			for _, ssm := range st.medias {
				var ip net.IP
				var rtpPort int
				var rtcpPort int

				if params, ok := st.MulticastParams[ssm.media]; ok {
					ip = params.IP
					rtpPort = params.RTPPort
					rtcpPort = params.RTCPPort
				} else {
					var err error
					ip, err = st.Server.getMulticastIP()
					if err != nil {
						return err
					}

					rtpPort = st.Server.MulticastRTPPort
					rtcpPort = st.Server.MulticastRTCPPort
				}

				smm := &serverMulticastWriterMedia{
					media:             ssm.media,
					maxPacketSize:     st.Server.MaxPacketSize,
					udpReadBufferSize: st.Server.UDPReadBufferSize,
					listenPacket:      st.Server.ListenPacket,
					writeQueueSize:    st.Server.WriteQueueSize,
					writeTimeout:      st.Server.WriteTimeout,
					ip:                ip,
					rtpPort:           rtpPort,
					rtcpPort:          rtcpPort,
					srtpOutCtx:        ssm.srtpOutCtx,
				}
				err := smm.initialize()
				if err != nil {
					return err
				}
				ssm.multicastWriter = smm

				for _, ssf := range ssm.formats {
					smf := &serverMulticastWriterFormat{
						senderReportPeriod:       st.Server.senderReportPeriod,
						timeNow:                  st.Server.timeNow,
						disableRTCPSenderReports: st.Server.DisableRTCPSenderReports,
						smm:                      smm,
						format:                   ssf.format,
					}
					smf.initialize()
					smm.formats[ssf.format.PayloadType()] = smf
					ssf.multicastWriter = smf
				}
			}
		}
		st.multicastReaderCount++
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

	st.readerRemoveUnsafe(ss)
}

func (st *ServerStream) readerRemoveUnsafe(ss *ServerSession) {
	delete(st.readers, ss)

	if ss.setuppedTransport.Protocol == ProtocolUDPMulticast {
		st.multicastReaderCount--
		if st.multicastReaderCount == 0 {
			for _, media := range st.medias {
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

	if ss.setuppedTransport.Protocol == ProtocolUDPMulticast {
		for medi, sm := range ss.setuppedMedias {
			streamMedia := st.medias[medi]
			streamMedia.multicastWriter.rtcpl.addClient(
				ss.author.ip(), streamMedia.multicastWriter.rtcpl.port(), sm.readPacketRTCPUDPPlay)
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

	st.readerSetInactiveUnsafe(ss)
}

func (st *ServerStream) readerSetInactiveUnsafe(ss *ServerSession) {
	if ss.setuppedTransport.Protocol == ProtocolUDPMulticast {
		for medi := range ss.setuppedMedias {
			streamMedia := st.medias[medi]
			streamMedia.multicastWriter.rtcpl.removeClient(ss.author.ip(), streamMedia.multicastWriter.rtcpl.port())
		}
	} else {
		delete(st.activeUnicastReaders, ss)
	}
}

// WritePacketRTP writes a RTP packet to all the readers of the stream.
func (st *ServerStream) WritePacketRTP(medi *description.Media, pkt *rtp.Packet) error {
	return st.WritePacketRTPWithNTP(medi, pkt, st.Server.timeNow())
}

// WritePacketRTPWithNTP writes a RTP packet to all the readers of the stream.
// ntp is the absolute timestamp of the packet, and is sent with periodic RTCP sender reports.
func (st *ServerStream) WritePacketRTPWithNTP(medi *description.Media, pkt *rtp.Packet, ntp time.Time) error {
	st.mutex.RLock()
	defer st.mutex.RUnlock()

	if st.closed {
		return liberrors.ErrServerStreamClosed{}
	}

	sm := st.medias[medi]
	sf := sm.formats[pkt.PayloadType]
	return sf.writePacketRTP(pkt, ntp)
}

// WritePacketRTCP writes a RTCP packet to all the readers of the stream.
func (st *ServerStream) WritePacketRTCP(medi *description.Media, pkt rtcp.Packet) error {
	st.mutex.RLock()
	defer st.mutex.RUnlock()

	if st.closed {
		return liberrors.ErrServerStreamClosed{}
	}

	sm := st.medias[medi]
	return sm.writePacketRTCP(pkt)
}
