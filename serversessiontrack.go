package gortsplib

import (
	"fmt"
	"time"

	"github.com/pion/rtp"

	"github.com/aler9/gortsplib/v2/pkg/format"
	"github.com/aler9/gortsplib/v2/pkg/rtcpreceiver"
	"github.com/aler9/gortsplib/v2/pkg/rtpreorderer"
)

type serverSessionFormat struct {
	sm              *serverSessionMedia
	format          format.Format
	udpReorderer    *rtpreorderer.Reorderer
	udpRTCPReceiver *rtcpreceiver.RTCPReceiver
	onPacketRTP     func(*rtp.Packet)
}

func newServerSessionFormat(sm *serverSessionMedia, forma format.Format) *serverSessionFormat {
	return &serverSessionFormat{
		sm:          sm,
		format:      forma,
		onPacketRTP: func(*rtp.Packet) {},
	}
}

func (st *serverSessionFormat) readRTPUDP(pkt *rtp.Packet, now time.Time) {
	packets, missing := st.udpReorderer.Process(pkt)
	if missing != 0 {
		onDecodeError(st.sm.ss, fmt.Errorf("%d RTP packet(s) lost", missing))
		// do not return
	}

	for _, pkt := range packets {
		st.udpRTCPReceiver.ProcessPacket(pkt, now, st.format.PTSEqualsDTS(pkt))
		st.onPacketRTP(pkt)
	}
}

func (st *serverSessionFormat) readRTPTCP(pkt *rtp.Packet) {
	st.onPacketRTP(pkt)
}
