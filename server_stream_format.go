package gortsplib

import (
	"crypto/rand"
	"sync/atomic"
	"time"

	"github.com/pion/rtcp"
	"github.com/pion/rtp"

	"github.com/bluenviron/gortsplib/v4/pkg/format"
	"github.com/bluenviron/gortsplib/v4/pkg/rtcpsender"
)

func randUint32() (uint32, error) {
	var b [4]byte
	_, err := rand.Read(b[:])
	if err != nil {
		return 0, err
	}
	return uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3]), nil
}

func isServerStreamLocalSSRCTaken(ssrc uint32, stream *ServerStream, exclude *serverStreamFormat) bool {
	for _, sm := range stream.medias {
		for _, sf := range sm.formats {
			if sf != exclude && sf.localSSRC == ssrc {
				return true
			}
		}
	}
	return false
}

func serverStreamPickLocalSSRC(sf *serverStreamFormat) (uint32, error) {
	for {
		ssrc, err := randUint32()
		if err != nil {
			return 0, err
		}

		if ssrc != 0 && !isServerStreamLocalSSRCTaken(ssrc, sf.sm.st, sf) {
			return ssrc, nil
		}
	}
}

type serverStreamFormat struct {
	sm     *serverStreamMedia
	format format.Format

	localSSRC      uint32
	rtcpSender     *rtcpsender.RTCPSender
	rtpPacketsSent *uint64
}

func (sf *serverStreamFormat) initialize() error {
	var err error
	sf.localSSRC, err = serverStreamPickLocalSSRC(sf)
	if err != nil {
		return err
	}

	sf.rtpPacketsSent = new(uint64)

	sf.rtcpSender = &rtcpsender.RTCPSender{
		ClockRate: sf.format.ClockRate(),
		Period:    sf.sm.st.Server.senderReportPeriod,
		TimeNow:   sf.sm.st.Server.timeNow,
		WritePacketRTCP: func(pkt rtcp.Packet) {
			if !sf.sm.st.Server.DisableRTCPSenderReports {
				sf.sm.st.WritePacketRTCP(sf.sm.media, pkt) //nolint:errcheck
			}
		},
	}
	sf.rtcpSender.Initialize()

	return nil
}

func (sf *serverStreamFormat) close() {
	if sf.rtcpSender != nil {
		sf.rtcpSender.Close()
	}
}

func (sf *serverStreamFormat) writePacketRTP(pkt *rtp.Packet, ntp time.Time) error {
	pkt.SSRC = sf.localSSRC

	byts := make([]byte, sf.sm.st.Server.MaxPacketSize)
	n, err := pkt.MarshalTo(byts)
	if err != nil {
		return err
	}
	byts = byts[:n]

	sf.rtcpSender.ProcessPacket(pkt, ntp, sf.format.PTSEqualsDTS(pkt))

	le := uint64(len(byts))

	// send unicast
	for r := range sf.sm.st.activeUnicastReaders {
		if _, ok := r.setuppedMedias[sf.sm.media]; ok {
			err := r.writePacketRTPEncoded(sf.sm.media, pkt.PayloadType, byts)
			if err != nil {
				r.onStreamWriteError(err)
				continue
			}

			atomic.AddUint64(sf.sm.bytesSent, le)
			atomic.AddUint64(sf.rtpPacketsSent, 1)
		}
	}

	// send multicast
	if sf.sm.multicastWriter != nil {
		err := sf.sm.multicastWriter.writePacketRTP(byts)
		if err != nil {
			return err
		}

		atomic.AddUint64(sf.sm.bytesSent, le)
		atomic.AddUint64(sf.rtpPacketsSent, 1)
	}

	return nil
}
