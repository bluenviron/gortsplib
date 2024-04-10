package gortsplib

import (
	"sync/atomic"
	"time"

	"github.com/pion/rtcp"
	"github.com/pion/rtp"

	"github.com/bluenviron/gortsplib/v4/pkg/format"
	"github.com/bluenviron/gortsplib/v4/pkg/rtcpsender"
)

type serverStreamFormat struct {
	sm     *serverStreamMedia
	format format.Format

	rtcpSender *rtcpsender.RTCPSender
}

func (sf *serverStreamFormat) initialize() {
	sf.rtcpSender = rtcpsender.New(
		sf.format.ClockRate(),
		sf.sm.st.s.senderReportPeriod,
		sf.sm.st.s.timeNow,
		func(pkt rtcp.Packet) {
			if !sf.sm.st.s.DisableRTCPSenderReports {
				sf.sm.st.WritePacketRTCP(sf.sm.media, pkt) //nolint:errcheck
			}
		},
	)
}

func (sf *serverStreamFormat) writePacketRTP(byts []byte, pkt *rtp.Packet, ntp time.Time) error {
	sf.rtcpSender.ProcessPacket(pkt, ntp, sf.format.PTSEqualsDTS(pkt))

	le := uint64(len(byts))

	// send unicast
	for r := range sf.sm.st.activeUnicastReaders {
		sm, ok := r.setuppedMedias[sf.sm.media]
		if ok {
			err := sm.writePacketRTP(byts)
			if err != nil {
				r.onStreamWriteError(err)
			} else {
				atomic.AddUint64(sf.sm.st.bytesSent, le)
			}
		}
	}

	// send multicast
	if sf.sm.multicastWriter != nil {
		err := sf.sm.multicastWriter.writePacketRTP(byts)
		if err != nil {
			return err
		}
		atomic.AddUint64(sf.sm.st.bytesSent, le)
	}

	return nil
}
