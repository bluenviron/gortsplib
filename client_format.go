package gortsplib

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pion/rtcp"
	"github.com/pion/rtp"

	"github.com/bluenviron/gortsplib/v5/pkg/format"
	"github.com/bluenviron/gortsplib/v5/pkg/liberrors"
	"github.com/bluenviron/gortsplib/v5/pkg/rtpreceiver"
	"github.com/bluenviron/gortsplib/v5/pkg/rtpsender"
)

type clientFormat struct {
	cm          *clientMedia
	format      format.Format
	localSSRC   uint32
	onPacketRTP OnPacketRTPFunc

	remoteSSRCMutex       sync.RWMutex          // play
	remoteSSRCFilled      bool                  // play
	remoteSSRCValue       uint32                // play
	rtpReceiver           *rtpreceiver.Receiver // play
	rtpSender             *rtpsender.Sender     // record or back channel
	writePacketRTPInQueue func([]byte) error
}

func (cf *clientFormat) initialize() {
	if cf.cm.udpRTPListener != nil {
		cf.writePacketRTPInQueue = cf.writePacketRTPInQueueUDP
	} else {
		cf.writePacketRTPInQueue = cf.writePacketRTPInQueueTCP
	}

	if cf.cm.c.state == clientStatePreRecord || cf.cm.media.IsBackChannel {
		cf.rtpSender = &rtpsender.Sender{
			ClockRate: cf.format.ClockRate(),
			Period:    cf.cm.c.senderReportPeriod,
			TimeNow:   cf.cm.c.timeNow,
			WritePacketRTCP: func(pkt rtcp.Packet) {
				if !cf.cm.c.DisableRTCPSenderReports {
					cf.cm.c.WritePacketRTCP(cf.cm.media, pkt) //nolint:errcheck
				}
			},
		}
		cf.rtpSender.Initialize()
	} else {
		cf.rtpReceiver = &rtpreceiver.Receiver{
			ClockRate:            cf.format.ClockRate(),
			LocalSSRC:            cf.localSSRC,
			UnrealiableTransport: (cf.cm.udpRTPListener != nil),
			Period:               cf.cm.c.receiverReportPeriod,
			TimeNow:              cf.cm.c.timeNow,
			WritePacketRTCP: func(pkt rtcp.Packet) {
				if cf.cm.udpRTPListener != nil && cf.cm.udpRTCPListener.writeAddr != nil {
					cf.cm.c.WritePacketRTCP(cf.cm.media, pkt) //nolint:errcheck
				}
			},
		}
		err := cf.rtpReceiver.Initialize()
		if err != nil {
			panic(err)
		}
	}
}

func (cf *clientFormat) close() {
	if cf.rtpReceiver != nil {
		cf.rtpReceiver.Close()
		cf.rtpReceiver = nil
	}
	if cf.rtpSender != nil {
		cf.rtpSender.Close()
		// do not set rtpSender to nil in order to preserve stats
	}
}

func (cf *clientFormat) stats() SessionStatsFormat { //nolint:dupl
	var recvStats *rtpreceiver.Stats
	if cf.rtpReceiver != nil {
		recvStats = cf.rtpReceiver.Stats()
	}

	var sentStats *rtpsender.Stats
	if cf.rtpSender != nil {
		sentStats = cf.rtpSender.Stats()
	}

	return SessionStatsFormat{
		InboundRTPPackets: func() uint64 {
			if recvStats != nil {
				return recvStats.Received
			}
			return 0
		}(),
		InboundRTPPacketsLost: func() uint64 {
			if recvStats != nil {
				return recvStats.Lost
			}
			return 0
		}(),
		InboundRTPPacketsJitter: func() float64 {
			if recvStats != nil {
				return recvStats.Jitter
			}
			return 0
		}(),
		InboundRTPPacketsLastSequenceNumber: func() uint16 {
			if recvStats != nil {
				return recvStats.LastSequenceNumber
			}
			return 0
		}(),
		InboundRTPPacketsLastRTP: func() uint32 {
			if recvStats != nil {
				return recvStats.LastRTP
			}
			return 0
		}(),
		InboundRTPPacketsLastNTP: func() time.Time {
			if recvStats != nil {
				return recvStats.LastNTP
			}
			return time.Time{}
		}(),
		OutboundRTPPackets: func() uint64 {
			if sentStats != nil {
				return sentStats.Sent
			}
			return 0
		}(),
		OutboundRTPPacketsReportedLost: func() uint64 {
			if sentStats != nil {
				return sentStats.ReportedLost
			}
			return 0
		}(),
		OutboundRTPPacketsLastSequenceNumber: func() uint16 {
			if sentStats != nil {
				return sentStats.LastSequenceNumber
			}
			return 0
		}(),
		OutboundRTPPacketsLastRTP: func() uint32 {
			if sentStats != nil {
				return sentStats.LastRTP
			}
			return 0
		}(),
		OutboundRTPPacketsLastNTP: func() time.Time {
			if sentStats != nil {
				return sentStats.LastNTP
			}
			return time.Time{}
		}(),
		LocalSSRC: cf.localSSRC,
		RemoteSSRC: func() uint32 {
			if v, ok := cf.remoteSSRC(); ok {
				return v
			}
			return 0
		}(),
		// deprecated
		RTPPacketsReceived: func() uint64 {
			if recvStats != nil {
				return recvStats.Received
			}
			return 0
		}(),
		RTPPacketsSent: func() uint64 {
			if sentStats != nil {
				return sentStats.Sent
			}
			return 0
		}(),
		RTPPacketsReportedLost: func() uint64 {
			if sentStats != nil {
				return sentStats.ReportedLost
			}
			return 0
		}(),
		RTPPacketsLost: func() uint64 {
			if recvStats != nil {
				return recvStats.Lost
			}
			return 0
		}(),
		RTPPacketsLastSequenceNumber: func() uint16 {
			if recvStats != nil {
				return recvStats.LastSequenceNumber
			}
			if sentStats != nil {
				return sentStats.LastSequenceNumber
			}
			return 0
		}(),
		RTPPacketsLastRTP: func() uint32 {
			if recvStats != nil {
				return recvStats.LastRTP
			}
			if sentStats != nil {
				return sentStats.LastRTP
			}
			return 0
		}(),
		RTPPacketsLastNTP: func() time.Time {
			if recvStats != nil {
				return recvStats.LastNTP
			}
			if sentStats != nil {
				return sentStats.LastNTP
			}
			return time.Time{}
		}(),
		RTPPacketsJitter: func() float64 {
			if recvStats != nil {
				return recvStats.Jitter
			}
			return 0
		}(),
	}
}

func (cf *clientFormat) remoteSSRC() (uint32, bool) {
	cf.remoteSSRCMutex.RLock()
	defer cf.remoteSSRCMutex.RUnlock()
	return cf.remoteSSRCValue, cf.remoteSSRCFilled
}

func (cf *clientFormat) readPacketRTP(payload []byte, header *rtp.Header, headerSize int, now time.Time) bool {
	if !cf.remoteSSRCFilled {
		cf.remoteSSRCMutex.Lock()
		cf.remoteSSRCFilled = true
		cf.remoteSSRCValue = header.SSRC
		cf.remoteSSRCMutex.Unlock()

		// a wrong SSRC is an issue only when encryption is enabled, since it spams srtp.Context.DecryptRTP.
	} else if cf.cm.srtpInCtx != nil &&
		header.SSRC != cf.remoteSSRCValue {
		cf.cm.onPacketRTPDecodeError(fmt.Errorf("received packet with wrong SSRC %d, expected %d",
			header.SSRC, cf.remoteSSRCValue))
		return false
	}

	pkt, err := cf.cm.decodeRTP(payload, header, headerSize)
	if err != nil {
		cf.cm.onPacketRTPDecodeError(err)
		return false
	}

	pkts, lost := cf.rtpReceiver.ProcessPacket2(pkt, now, cf.format.PTSEqualsDTS(pkt))

	if lost != 0 {
		cf.cm.c.OnPacketsLost(lost)
	}

	for _, pkt := range pkts {
		cf.onPacketRTP(pkt)
	}

	return true
}

func (cf *clientFormat) writePacketRTP(pkt *rtp.Packet, ntp time.Time) error {
	pkt.SSRC = cf.localSSRC

	maxPlainPacketSize := cf.cm.c.MaxPacketSize
	if cf.cm.srtpOutCtx != nil {
		maxPlainPacketSize -= srtpOverhead
	}

	plain := make([]byte, maxPlainPacketSize)
	n, err := pkt.MarshalTo(plain)
	if err != nil {
		return err
	}
	plain = plain[:n]

	var encr []byte
	if cf.cm.srtpOutCtx != nil {
		encr = make([]byte, cf.cm.c.MaxPacketSize)
		encr, err = cf.cm.srtpOutCtx.encryptRTP(encr, plain, &pkt.Header)
		if err != nil {
			return err
		}
	}

	ptsEqualsDTS := cf.format.PTSEqualsDTS(pkt)

	if cf.cm.srtpOutCtx != nil {
		return cf.writePacketRTPEncoded(pkt, ntp, ptsEqualsDTS, encr)
	}
	return cf.writePacketRTPEncoded(pkt, ntp, ptsEqualsDTS, plain)
}

func (cf *clientFormat) writePacketRTPEncoded(
	pkt *rtp.Packet,
	ntp time.Time,
	ptsEqualsDTS bool,
	payload []byte,
) error {
	cf.rtpSender.ProcessPacket(pkt, ntp, ptsEqualsDTS)

	atomic.AddUint64(cf.cm.bytesSent, uint64(len(payload)))

	cf.cm.c.writerMutex.RLock()
	defer cf.cm.c.writerMutex.RUnlock()

	if cf.cm.c.writer == nil {
		return nil
	}

	ok := cf.cm.c.writer.Push(func() error {
		return cf.writePacketRTPInQueue(payload)
	})
	if !ok {
		return liberrors.ErrClientWriteQueueFull{}
	}

	return nil
}

func (cf *clientFormat) writePacketRTPInQueueUDP(payload []byte) error {
	return cf.cm.udpRTPListener.write(payload)
}

func (cf *clientFormat) writePacketRTPInQueueTCP(payload []byte) error {
	cf.cm.c.tcpFrame.Channel = cf.cm.tcpChannel
	cf.cm.c.tcpFrame.Payload = payload
	cf.cm.c.nconn.SetWriteDeadline(time.Now().Add(cf.cm.c.WriteTimeout))
	return cf.cm.c.conn.WriteInterleavedFrame(cf.cm.c.tcpFrame, cf.cm.c.tcpBuffer)
}
