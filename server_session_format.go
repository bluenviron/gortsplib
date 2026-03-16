package gortsplib

import (
	"fmt"
	"log"
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

type serverSessionFormat struct {
	ssm         *serverSessionMedia
	format      format.Format
	localSSRC   uint32
	onPacketRTP OnPacketRTPFunc

	remoteSSRCMutex       sync.RWMutex          // record or back channel
	remoteSSRCFilled      bool                  // record or back channel
	remoteSSRCValue       uint32                // record or back channel
	rtpReceiver           *rtpreceiver.Receiver // record or back channel
	writePacketRTPInQueue func([]byte) error
	rtpSender             *rtpsender.Sender // play
}

func (ssf *serverSessionFormat) initialize() {
	udp := ssf.ssm.ss.setuppedTransport.Protocol == ProtocolUDP ||
		ssf.ssm.ss.setuppedTransport.Protocol == ProtocolUDPMulticast

	if udp {
		ssf.writePacketRTPInQueue = ssf.writePacketRTPInQueueUDP
	} else {
		ssf.writePacketRTPInQueue = ssf.writePacketRTPInQueueTCP
	}

	if ssf.ssm.ss.state == ServerSessionStatePreRecord || ssf.ssm.media.IsBackChannel {
		ssf.rtpReceiver = &rtpreceiver.Receiver{
			ClockRate:            ssf.format.ClockRate(),
			LocalSSRC:            ssf.localSSRC,
			UnrealiableTransport: udp,
			Period:               ssf.ssm.ss.s.receiverReportPeriod,
			TimeNow:              ssf.ssm.ss.s.timeNow,
			WritePacketRTCP: func(pkt rtcp.Packet) {
				if udp {
					ssf.ssm.writePacketRTCP(pkt) //nolint:errcheck
				}
			},
		}
		err := ssf.rtpReceiver.Initialize()
		if err != nil {
			panic(err)
		}
	} else {
		ssf.rtpSender = &rtpsender.Sender{
			ClockRate: ssf.format.ClockRate(),
			Period:    ssf.ssm.ss.s.senderReportPeriod,
			TimeNow:   ssf.ssm.ss.s.timeNow,
			WritePacketRTCP: func(pkt rtcp.Packet) {
				if !ssf.ssm.ss.s.DisableRTCPSenderReports {
					ssf.ssm.writePacketRTCP(pkt) //nolint:errcheck
				}
			},
		}
		ssf.rtpSender.Initialize()
	}
}

func (ssf *serverSessionFormat) close() {
	if ssf.rtpReceiver != nil {
		ssf.rtpReceiver.Close()
		ssf.rtpReceiver = nil
	}
	if ssf.rtpSender != nil {
		ssf.rtpSender.Close()
		// do not set rtpSender to nil in order to preserve stats
	}
}

func (ssf *serverSessionFormat) stats() SessionStatsFormat { //nolint:dupl
	var recvStats *rtpreceiver.Stats
	if ssf.rtpReceiver != nil {
		recvStats = ssf.rtpReceiver.Stats()
	}

	var sentStats *rtpsender.Stats
	if ssf.rtpSender != nil {
		sentStats = ssf.rtpSender.Stats()
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
		LocalSSRC: ssf.localSSRC,
		RemoteSSRC: func() uint32 {
			if v, ok := ssf.remoteSSRC(); ok {
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

func (ssf *serverSessionFormat) remoteSSRC() (uint32, bool) {
	ssf.remoteSSRCMutex.RLock()
	defer ssf.remoteSSRCMutex.RUnlock()
	return ssf.remoteSSRCValue, ssf.remoteSSRCFilled
}

func (ssf *serverSessionFormat) readPacketRTP(payload []byte, header *rtp.Header, headerSize int, now time.Time) bool {
	if !ssf.remoteSSRCFilled {
		ssf.remoteSSRCMutex.Lock()
		ssf.remoteSSRCFilled = true
		ssf.remoteSSRCValue = header.SSRC
		ssf.remoteSSRCMutex.Unlock()

		// a wrong SSRC is an issue only when encryption is enabled, since it spams srtp.Context.DecryptRTP.
	} else if ssf.ssm.srtpInCtx != nil &&
		header.SSRC != ssf.remoteSSRCValue {
		ssf.ssm.onPacketRTPDecodeError(fmt.Errorf("received packet with wrong SSRC %d, expected %d",
			header.SSRC, ssf.remoteSSRCValue))
		return false
	}

	pkt, err := ssf.ssm.decodeRTP(payload, header, headerSize)
	if err != nil {
		ssf.ssm.onPacketRTPDecodeError(err)
		return false
	}

	pkts, lost := ssf.rtpReceiver.ProcessPacket2(pkt, now, ssf.format.PTSEqualsDTS(pkt))

	if lost != 0 {
		if h, ok := ssf.ssm.ss.s.Handler.(ServerHandlerOnPacketsLost); ok {
			h.OnPacketsLost(&ServerHandlerOnPacketsLostCtx{
				Session: ssf.ssm.ss,
				Lost:    lost,
			})
		} else {
			log.Printf("%d RTP %s lost",
				lost,
				func() string {
					if lost == 1 {
						return "packet"
					}
					return "packets"
				}())
		}
	}

	for _, pkt := range pkts {
		ssf.onPacketRTP(pkt)
	}

	return true
}

func (ssf *serverSessionFormat) writePacketRTP(pkt *rtp.Packet, ntp time.Time) error {
	pkt.SSRC = ssf.localSSRC

	maxPlainPacketSize := ssf.ssm.ss.s.MaxPacketSize
	if ssf.ssm.srtpOutCtx != nil {
		maxPlainPacketSize -= srtpOverhead
	}

	plain := make([]byte, maxPlainPacketSize)
	n, err := pkt.MarshalTo(plain)
	if err != nil {
		return err
	}
	plain = plain[:n]

	var encr []byte
	if ssf.ssm.srtpOutCtx != nil {
		encr = make([]byte, ssf.ssm.ss.s.MaxPacketSize)
		encr, err = ssf.ssm.srtpOutCtx.encryptRTP(encr, plain, &pkt.Header)
		if err != nil {
			return err
		}
	}

	ptsEqualsDTS := ssf.format.PTSEqualsDTS(pkt)

	if ssf.ssm.srtpOutCtx != nil {
		return ssf.writePacketRTPEncoded(pkt, ntp, ptsEqualsDTS, encr)
	}
	return ssf.writePacketRTPEncoded(pkt, ntp, ptsEqualsDTS, plain)
}

func (ssf *serverSessionFormat) writePacketRTPEncoded(
	pkt *rtp.Packet,
	ntp time.Time,
	ptsEqualsDTS bool,
	payload []byte,
) error {
	ssf.rtpSender.ProcessPacket(pkt, ntp, ptsEqualsDTS)

	atomic.AddUint64(ssf.ssm.bytesSent, uint64(len(payload)))

	ssf.ssm.ss.writerMutex.RLock()
	defer ssf.ssm.ss.writerMutex.RUnlock()

	if ssf.ssm.ss.writer == nil {
		return nil
	}

	ok := ssf.ssm.ss.writer.Push(func() error {
		return ssf.writePacketRTPInQueue(payload)
	})
	if !ok {
		return liberrors.ErrServerWriteQueueFull{}
	}

	return nil
}

func (ssf *serverSessionFormat) writePacketRTPInQueueUDP(payload []byte) error {
	return ssf.ssm.ss.s.udpRTPListener.write(payload, ssf.ssm.udpRTPWriteAddr)
}

func (ssf *serverSessionFormat) writePacketRTPInQueueTCP(payload []byte) error {
	ssf.ssm.ss.tcpFrame.Channel = ssf.ssm.tcpChannel
	ssf.ssm.ss.tcpFrame.Payload = payload
	ssf.ssm.ss.tcpConn.nconn.SetWriteDeadline(time.Now().Add(ssf.ssm.ss.s.WriteTimeout))
	return ssf.ssm.ss.tcpConn.conn.WriteInterleavedFrame(ssf.ssm.ss.tcpFrame, ssf.ssm.ss.tcpBuffer)
}
