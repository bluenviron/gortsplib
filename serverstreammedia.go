package gortsplib

import (
	"time"

	"github.com/pion/rtcp"
	"github.com/pion/rtp"

	"github.com/aler9/gortsplib/v2/pkg/media"
)

type serverStreamMedia struct {
	media            *media.Media
	formats          map[uint8]*serverStreamFormat
	multicastHandler *serverMulticastHandler
}

func newServerStreamMedia(medi *media.Media) *serverStreamMedia {
	return &serverStreamMedia{
		media: medi,
	}
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
