package gortsplib

import (
	"crypto/rand"
	"slices"
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

func serverStreamPickLocalSSRC(sf *serverStreamFormat) (uint32, error) {
	var takenSSRCs []uint32 //nolint:prealloc

	for _, sm := range sf.sm.st.medias {
		for _, sf := range sm.formats {
			takenSSRCs = append(takenSSRCs, sf.localSSRC)
		}
	}

	for _, sf := range sf.sm.formats {
		takenSSRCs = append(takenSSRCs, sf.localSSRC)
	}

	for {
		ssrc, err := randUint32()
		if err != nil {
			return 0, err
		}

		if ssrc != 0 && !slices.Contains(takenSSRCs, ssrc) {
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

	sf.rtcpSender.ProcessPacket(pkt, ntp, sf.format.PTSEqualsDTS(pkt))

	maxPlainPacketSize := sf.sm.st.Server.MaxPacketSize
	if sf.sm.srtpOutCtx != nil {
		maxPlainPacketSize -= srtpOverhead
	}

	plain := make([]byte, maxPlainPacketSize)
	n, err := pkt.MarshalTo(plain)
	if err != nil {
		return err
	}
	plain = plain[:n]

	var encr []byte
	if sf.sm.srtpOutCtx != nil {
		encr = make([]byte, sf.sm.st.Server.MaxPacketSize)
		encr, err = sf.sm.srtpOutCtx.encryptRTP(encr, plain, &pkt.Header)
		if err != nil {
			return err
		}
	}

	encrLen := uint64(len(encr))
	plainLen := uint64(len(plain))

	// send unicast
	for r := range sf.sm.st.activeUnicastReaders {
		if rsm, ok := r.setuppedMedias[sf.sm.media]; ok {
			rsf := rsm.formats[pkt.PayloadType]

			if r.setuppedSecure {
				err := rsf.writePacketRTPEncoded(encr)
				if err != nil {
					r.onStreamWriteError(err)
					continue
				}

				atomic.AddUint64(sf.sm.bytesSent, encrLen)
			} else {
				err := rsf.writePacketRTPEncoded(plain)
				if err != nil {
					r.onStreamWriteError(err)
					continue
				}

				atomic.AddUint64(sf.sm.bytesSent, plainLen)
			}

			atomic.AddUint64(sf.rtpPacketsSent, 1)
		}
	}

	// send multicast
	if sf.sm.multicastWriter != nil {
		if sf.sm.srtpOutCtx != nil {
			err := sf.sm.multicastWriter.writePacketRTP(encr)
			if err != nil {
				return err
			}

			atomic.AddUint64(sf.sm.bytesSent, encrLen)
		} else {
			err := sf.sm.multicastWriter.writePacketRTP(plain)
			if err != nil {
				return err
			}

			atomic.AddUint64(sf.sm.bytesSent, plainLen)
		}

		atomic.AddUint64(sf.rtpPacketsSent, 1)
	}

	return nil
}
