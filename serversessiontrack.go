package gortsplib

import (
	"fmt"
	"time"

	"github.com/pion/rtp"

	"github.com/aler9/gortsplib/v2/pkg/rtcpreceiver"
	"github.com/aler9/gortsplib/v2/pkg/rtpreorderer"
	"github.com/aler9/gortsplib/v2/pkg/track"
)

type serverSessionTrack struct {
	sm              *serverSessionMedia
	track           track.Track
	udpReorderer    *rtpreorderer.Reorderer
	udpRTCPReceiver *rtcpreceiver.RTCPReceiver
	onPacketRTP     func(*rtp.Packet)
}

func newServerSessionTrack(sm *serverSessionMedia, trak track.Track) *serverSessionTrack {
	return &serverSessionTrack{
		sm:          sm,
		track:       trak,
		onPacketRTP: func(*rtp.Packet) {},
	}
}

func (st *serverSessionTrack) readRTPUDP(pkt *rtp.Packet, now time.Time) {
	packets, missing := st.udpReorderer.Process(pkt)
	if missing != 0 {
		onDecodeError(st.sm.ss, fmt.Errorf("%d RTP packet(s) lost", missing))
		// do not return
	}

	for _, pkt := range packets {
		st.udpRTCPReceiver.ProcessPacket(pkt, now, st.track.PTSEqualsDTS(pkt))
		st.onPacketRTP(pkt)
	}
}

func (st *serverSessionTrack) readRTPTCP(pkt *rtp.Packet) {
	st.onPacketRTP(pkt)
}
