package gortsplib

import (
	"log"
	"sync/atomic"
	"time"

	"github.com/pion/rtcp"
	"github.com/pion/rtp"

	"github.com/bluenviron/gortsplib/v5/pkg/format"
	"github.com/bluenviron/gortsplib/v5/pkg/liberrors"
	"github.com/bluenviron/gortsplib/v5/pkg/rtpreceiver"
)

type serverSessionFormat struct {
	sm          *serverSessionMedia
	format      format.Format
	localSSRC   uint32
	onPacketRTP OnPacketRTPFunc

	rtpReceiver           *rtpreceiver.Receiver
	writePacketRTPInQueue func([]byte) error
	rtpPacketsReceived    *uint64
	rtpPacketsSent        *uint64
	rtpPacketsLost        *uint64
}

func (sf *serverSessionFormat) initialize() {
	sf.rtpPacketsReceived = new(uint64)
	sf.rtpPacketsSent = new(uint64)
	sf.rtpPacketsLost = new(uint64)

	udp := sf.sm.ss.setuppedTransport.Protocol == ProtocolUDP ||
		sf.sm.ss.setuppedTransport.Protocol == ProtocolUDPMulticast

	if udp {
		sf.writePacketRTPInQueue = sf.writePacketRTPInQueueUDP
	} else {
		sf.writePacketRTPInQueue = sf.writePacketRTPInQueueTCP
	}

	if sf.sm.ss.state == ServerSessionStatePreRecord || sf.sm.media.IsBackChannel {
		sf.rtpReceiver = &rtpreceiver.Receiver{
			ClockRate:            sf.format.ClockRate(),
			LocalSSRC:            sf.localSSRC,
			UnrealiableTransport: udp,
			Period:               sf.sm.ss.s.receiverReportPeriod,
			TimeNow:              sf.sm.ss.s.timeNow,
			WritePacketRTCP: func(pkt rtcp.Packet) {
				if udp {
					sf.sm.ss.WritePacketRTCP(sf.sm.media, pkt) //nolint:errcheck
				}
			},
		}
		err := sf.rtpReceiver.Initialize()
		if err != nil {
			panic(err)
		}
	}
}

func (sf *serverSessionFormat) close() {
	if sf.rtpReceiver != nil {
		sf.rtpReceiver.Close()
		sf.rtpReceiver = nil
	}
}

func (sf *serverSessionFormat) remoteSSRC() (uint32, bool) {
	if sf.rtpReceiver != nil {
		stats := sf.rtpReceiver.Stats()
		if stats != nil {
			return stats.RemoteSSRC, true
		}
	}
	return 0, false
}

func (sf *serverSessionFormat) readPacketRTP(pkt *rtp.Packet, now time.Time) {
	pkts, lost, err := sf.rtpReceiver.ProcessPacket(pkt, now, sf.format.PTSEqualsDTS(pkt))
	if err != nil {
		sf.sm.onPacketRTPDecodeError(err)
		return
	}

	if lost != 0 {
		atomic.AddUint64(sf.rtpPacketsLost, lost)

		if h, ok := sf.sm.ss.s.Handler.(ServerHandlerOnPacketsLost); ok {
			h.OnPacketsLost(&ServerHandlerOnPacketsLostCtx{
				Session: sf.sm.ss,
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

	atomic.AddUint64(sf.rtpPacketsReceived, uint64(len(pkts)))

	for _, pkt := range pkts {
		sf.onPacketRTP(pkt)
	}
}

func (sf *serverSessionFormat) writePacketRTP(pkt *rtp.Packet) error {
	pkt.SSRC = sf.localSSRC

	maxPlainPacketSize := sf.sm.ss.s.MaxPacketSize
	if isSecure(sf.sm.ss.setuppedTransport.Profile) {
		maxPlainPacketSize -= srtpOverhead
	}

	plain := make([]byte, maxPlainPacketSize)
	n, err := pkt.MarshalTo(plain)
	if err != nil {
		return err
	}
	plain = plain[:n]

	var encr []byte
	if isSecure(sf.sm.ss.setuppedTransport.Profile) {
		encr = make([]byte, sf.sm.ss.s.MaxPacketSize)
		encr, err = sf.sm.srtpOutCtx.encryptRTP(encr, plain, &pkt.Header)
		if err != nil {
			return err
		}
	}

	if isSecure(sf.sm.ss.setuppedTransport.Profile) {
		return sf.writePacketRTPEncoded(encr)
	}
	return sf.writePacketRTPEncoded(plain)
}

func (sf *serverSessionFormat) writePacketRTPEncoded(payload []byte) error {
	sf.sm.ss.writerMutex.RLock()
	defer sf.sm.ss.writerMutex.RUnlock()

	if sf.sm.ss.writer == nil {
		return nil
	}

	ok := sf.sm.ss.writer.Push(func() error {
		return sf.writePacketRTPInQueue(payload)
	})
	if !ok {
		return liberrors.ErrServerWriteQueueFull{}
	}

	return nil
}

func (sf *serverSessionFormat) writePacketRTPInQueueUDP(payload []byte) error {
	err := sf.sm.ss.s.udpRTPListener.write(payload, sf.sm.udpRTPWriteAddr)
	if err != nil {
		return err
	}

	atomic.AddUint64(sf.sm.bytesSent, uint64(len(payload)))
	atomic.AddUint64(sf.rtpPacketsSent, 1)
	return nil
}

func (sf *serverSessionFormat) writePacketRTPInQueueTCP(payload []byte) error {
	sf.sm.ss.tcpFrame.Channel = sf.sm.tcpChannel
	sf.sm.ss.tcpFrame.Payload = payload
	sf.sm.ss.tcpConn.nconn.SetWriteDeadline(time.Now().Add(sf.sm.ss.s.WriteTimeout))
	err := sf.sm.ss.tcpConn.conn.WriteInterleavedFrame(sf.sm.ss.tcpFrame, sf.sm.ss.tcpBuffer)
	if err != nil {
		return err
	}

	atomic.AddUint64(sf.sm.bytesSent, uint64(len(payload)))
	atomic.AddUint64(sf.rtpPacketsSent, 1)
	return nil
}
