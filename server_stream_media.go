package gortsplib

import (
	"fmt"
	"sync/atomic"
	"time"

	"github.com/bluenviron/gortsplib/v5/pkg/description"
	"github.com/bluenviron/gortsplib/v5/pkg/format"
	"github.com/bluenviron/gortsplib/v5/pkg/headers"
	"github.com/pion/rtcp"
)

type serverStreamMedia struct {
	st         *ServerStream
	media      *description.Media
	trackID    int
	localSSRCs map[uint8]uint32
	srtpOutCtx *wrappedSRTPContext

	formats         map[uint8]*serverStreamFormat
	multicastWriter *serverMulticastWriterMedia
	bytesSent       atomic.Uint64
	rtcpPacketsSent atomic.Uint64
}

func (ssm *serverStreamMedia) initialize() {
	ssm.formats = make(map[uint8]*serverStreamFormat)

	for _, forma := range ssm.media.Formats {
		sf := &serverStreamFormat{
			ssm:       ssm,
			format:    forma,
			localSSRC: ssm.localSSRCs[forma.PayloadType()],
		}
		sf.initialize()
		ssm.formats[forma.PayloadType()] = sf
	}
}

func (ssm *serverStreamMedia) rtpInfoEntry(now time.Time) *headers.RTPInfoEntry {
	// do not generate a RTP-Info entry when
	// there are multiple formats inside a single media stream,
	// since RTP-Info does not support multiple sequence numbers / timestamps.
	if len(ssm.media.Formats) > 1 {
		return nil
	}

	return ssm.formats[ssm.media.Formats[0].PayloadType()].rtpInfoEntry(now)
}

func (ssm *serverStreamMedia) stats() ServerStreamStatsMedia {
	bytesSent := ssm.bytesSent.Load()
	rtcpPacketsSent := ssm.rtcpPacketsSent.Load()

	return ServerStreamStatsMedia{
		OutboundBytes:       bytesSent,
		OutboundRTCPPackets: rtcpPacketsSent,
		Formats: func() map[format.Format]ServerStreamStatsFormat {
			ret := make(map[format.Format]ServerStreamStatsFormat)
			for _, ssf := range ssm.formats {
				ret[ssf.format] = ssf.stats()
			}
			return ret
		}(),
		// deprecated
		BytesSent:       bytesSent,
		RTCPPacketsSent: rtcpPacketsSent,
	}
}

func (ssm *serverStreamMedia) writePacketRTCP(pkt rtcp.Packet) error {
	plain, err := pkt.Marshal()
	if err != nil {
		return err
	}

	maxPlainPacketSize := ssm.st.Server.MaxPacketSize
	if ssm.srtpOutCtx != nil {
		maxPlainPacketSize -= srtcpOverhead
	}

	if len(plain) > maxPlainPacketSize {
		return fmt.Errorf("packet is too big")
	}

	var encr []byte
	if ssm.srtpOutCtx != nil {
		encr = make([]byte, ssm.st.Server.MaxPacketSize)
		encr, err = ssm.srtpOutCtx.encryptRTCP(encr, plain, nil)
		if err != nil {
			return err
		}
	}

	encrLen := uint64(len(encr))
	plainLen := uint64(len(plain))

	// send unicast
	for r := range ssm.st.activeUnicastReaders {
		if sm, ok := r.setuppedMedias[ssm.media]; ok {
			if sm.srtpOutCtx != nil {
				err = sm.writePacketRTCPEncoded(encr)
				if err != nil {
					r.onStreamWriteError(err)
					continue
				}

				ssm.bytesSent.Add(encrLen)
			} else {
				err = sm.writePacketRTCPEncoded(plain)
				if err != nil {
					r.onStreamWriteError(err)
					continue
				}

				ssm.bytesSent.Add(plainLen)
			}

			ssm.rtcpPacketsSent.Add(1)
		}
	}

	// send multicast
	if ssm.multicastWriter != nil {
		if ssm.srtpOutCtx != nil {
			err = ssm.multicastWriter.writePacketRTCPEncoded(encr)
			if err != nil {
				return err
			}

			ssm.bytesSent.Add(encrLen)
		} else {
			err = ssm.multicastWriter.writePacketRTCPEncoded(plain)
			if err != nil {
				return err
			}

			ssm.bytesSent.Add(plainLen)
		}

		ssm.rtcpPacketsSent.Add(1)
	}

	return nil
}
