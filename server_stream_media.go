package gortsplib

import (
	"time"

	"github.com/google/uuid"
	"github.com/pion/rtcp"
	"github.com/pion/rtp"

	"github.com/aler9/gortsplib/v2/pkg/media"
	"github.com/aler9/gortsplib/v2/pkg/rtcpsender"
)

type serverStreamMedia struct {
	uuid             uuid.UUID
	media            *media.Media
	formats          map[uint8]*serverStreamFormat
	multicastHandler *serverMulticastHandler
}

func newServerStreamMedia(st *ServerStream, medi *media.Media) *serverStreamMedia {
	sm := &serverStreamMedia{
		uuid:  uuid.New(),
		media: medi,
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
				st.WritePacketRTCP(cmedia, pkt)
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

	if sm.multicastHandler != nil {
		sm.multicastHandler.close()
	}
}

func (sm *serverStreamMedia) allocateMulticastHandler(s *Server) error {
	if sm.multicastHandler == nil {
		mh, err := newServerMulticastHandler(s)
		if err != nil {
			return err
		}

		sm.multicastHandler = mh
	}
	return nil
}

func (sm *serverStreamMedia) WritePacketRTPWithNTP(ss *ServerStream, pkt *rtp.Packet, ntp time.Time) {
	byts := make([]byte, maxPacketSize)
	n, err := pkt.MarshalTo(byts)
	if err != nil {
		return
	}
	byts = byts[:n]

	forma := sm.formats[pkt.PayloadType]

	forma.rtcpSender.ProcessPacket(pkt, ntp, forma.format.PTSEqualsDTS(pkt))

	// send unicast
	for r := range ss.activeUnicastReaders {
		sm, ok := r.setuppedMedias[sm.media]
		if ok {
			sm.writePacketRTP(byts)
		}
	}

	// send multicast
	if sm.multicastHandler != nil {
		sm.multicastHandler.writePacketRTP(byts)
	}
}

func (sm *serverStreamMedia) writePacketRTCP(ss *ServerStream, pkt rtcp.Packet) {
	byts, err := pkt.Marshal()
	if err != nil {
		return
	}

	// send unicast
	for r := range ss.activeUnicastReaders {
		sm, ok := r.setuppedMedias[sm.media]
		if ok {
			sm.writePacketRTCP(byts)
		}
	}

	// send multicast
	if sm.multicastHandler != nil {
		sm.multicastHandler.writePacketRTCP(byts)
	}
}
