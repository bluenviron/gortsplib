package gortsplib

import (
	"time"

	"github.com/pion/rtcp"
	"github.com/pion/rtp"

	"github.com/bluenviron/gortsplib/v3/pkg/media"
	"github.com/bluenviron/gortsplib/v3/pkg/rtcpsender"
)

type serverStreamMedia struct {
	st              *ServerStream
	media           *media.Media
	trackID         int
	formats         map[uint8]*serverStreamFormat
	multicastWriter *serverMulticastWriter
}

func newServerStreamMedia(st *ServerStream, medi *media.Media, trackID int) *serverStreamMedia {
	sm := &serverStreamMedia{
		st:      st,
		media:   medi,
		trackID: trackID,
	}

	sm.formats = make(map[uint8]*serverStreamFormat)
	for _, forma := range medi.Formats {
		tr := &serverStreamFormat{
			format: forma,
		}

		cmedia := medi
		tr.rtcpSender = rtcpsender.New(
			forma.ClockRate(),
			func(pkt rtcp.Packet) {
				st.WritePacketRTCP(cmedia, pkt) //nolint:errcheck
			},
		)

		sm.formats[forma.PayloadType()] = tr
	}

	return sm
}

func (sm *serverStreamMedia) close() {
	for _, tr := range sm.formats {
		if tr.rtcpSender != nil {
			tr.rtcpSender.Close()
		}
	}

	if sm.multicastWriter != nil {
		sm.multicastWriter.close()
	}
}

func (sm *serverStreamMedia) allocateMulticastHandler(s *Server) error {
	if sm.multicastWriter == nil {
		mh, err := newServerMulticastWriter(s)
		if err != nil {
			return err
		}

		sm.multicastWriter = mh
	}
	return nil
}

func (sm *serverStreamMedia) writePacketRTP(byts []byte, pkt *rtp.Packet, ntp time.Time) {
	forma := sm.formats[pkt.PayloadType]

	forma.rtcpSender.ProcessPacket(pkt, ntp, forma.format.PTSEqualsDTS(pkt))

	// send unicast
	for r := range sm.st.activeUnicastReaders {
		sm, ok := r.setuppedMedias[sm.media]
		if ok {
			sm.writePacketRTP(byts)
		}
	}

	// send multicast
	if sm.multicastWriter != nil {
		sm.multicastWriter.writePacketRTP(byts)
	}
}

func (sm *serverStreamMedia) writePacketRTCP(byts []byte) {
	// send unicast
	for r := range sm.st.activeUnicastReaders {
		sm, ok := r.setuppedMedias[sm.media]
		if ok {
			sm.writePacketRTCP(byts)
		}
	}

	// send multicast
	if sm.multicastWriter != nil {
		sm.multicastWriter.writePacketRTCP(byts)
	}
}
