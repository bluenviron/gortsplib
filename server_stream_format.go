package gortsplib

import (
	"crypto/rand"
	"sync/atomic"
	"time"

	"github.com/pion/rtcp"
	"github.com/pion/rtp"

	"github.com/bluenviron/gortsplib/v5/pkg/format"
	"github.com/bluenviron/gortsplib/v5/pkg/rtpsender"
)

func randUint32() (uint32, error) {
	var b [4]byte
	_, err := rand.Read(b[:])
	if err != nil {
		return 0, err
	}
	return uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3]), nil
}

type serverStreamFormat struct {
	sm        *serverStreamMedia
	format    format.Format
	localSSRC uint32

	rtpSender      *rtpsender.Sender
	rtpPacketsSent *uint64
}

func (sf *serverStreamFormat) initialize() {
	sf.rtpPacketsSent = new(uint64)

	sf.rtpSender = &rtpsender.Sender{
		ClockRate: sf.format.ClockRate(),
		Period:    sf.sm.st.Server.senderReportPeriod,
		TimeNow:   sf.sm.st.Server.timeNow,
		WritePacketRTCP: func(pkt rtcp.Packet) {
			if !sf.sm.st.Server.DisableRTCPSenderReports {
				sf.sm.st.WritePacketRTCP(sf.sm.media, pkt) //nolint:errcheck
			}
		},
	}
	sf.rtpSender.Initialize()
}

func (sf *serverStreamFormat) close() {
	if sf.rtpSender != nil {
		sf.rtpSender.Close()
	}
}

func (sf *serverStreamFormat) writePacketRTP(pkt *rtp.Packet, ntp time.Time) error {
	pkt.SSRC = sf.localSSRC

	sf.rtpSender.ProcessPacket(pkt, ntp, sf.format.PTSEqualsDTS(pkt))

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

			if isSecure(r.setuppedTransport.Profile) {
				err = rsf.writePacketRTPEncoded(encr)
				if err != nil {
					r.onStreamWriteError(err)
					continue
				}

				atomic.AddUint64(sf.sm.bytesSent, encrLen)
			} else {
				err = rsf.writePacketRTPEncoded(plain)
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
			err = sf.sm.multicastWriter.writePacketRTP(encr)
			if err != nil {
				return err
			}

			atomic.AddUint64(sf.sm.bytesSent, encrLen)
		} else {
			err = sf.sm.multicastWriter.writePacketRTP(plain)
			if err != nil {
				return err
			}

			atomic.AddUint64(sf.sm.bytesSent, plainLen)
		}

		atomic.AddUint64(sf.rtpPacketsSent, 1)
	}

	return nil
}
