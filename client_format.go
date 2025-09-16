package gortsplib

import (
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

	rtpReceiver           *rtpreceiver.Receiver // play
	rtpSender             *rtpsender.Sender     // record or back channel
	writePacketRTPInQueue func([]byte) error
	rtpPacketsReceived    *uint64
	rtpPacketsSent        *uint64
	rtpPacketsLost        *uint64
}

func (cf *clientFormat) initialize() {
	cf.rtpPacketsReceived = new(uint64)
	cf.rtpPacketsSent = new(uint64)
	cf.rtpPacketsLost = new(uint64)

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
	}
}

func (cf *clientFormat) remoteSSRC() (uint32, bool) {
	if cf.rtpReceiver != nil {
		stats := cf.rtpReceiver.Stats()
		if stats != nil {
			return stats.RemoteSSRC, true
		}
	}
	return 0, false
}

func (cf *clientFormat) readPacketRTP(pkt *rtp.Packet) {
	now := cf.cm.c.timeNow()

	pkts, lost, err := cf.rtpReceiver.ProcessPacket(pkt, now, cf.format.PTSEqualsDTS(pkt))
	if err != nil {
		cf.cm.onPacketRTPDecodeError(err)
		return
	}

	if lost != 0 {
		atomic.AddUint64(cf.rtpPacketsLost, lost)
		cf.cm.c.OnPacketsLost(lost)
	}

	atomic.AddUint64(cf.rtpPacketsReceived, uint64(len(pkts)))

	for _, pkt := range pkts {
		cf.onPacketRTP(pkt)
	}
}

func (cf *clientFormat) writePacketRTP(pkt *rtp.Packet, ntp time.Time) error {
	pkt.SSRC = cf.localSSRC

	cf.rtpSender.ProcessPacket(pkt, ntp, cf.format.PTSEqualsDTS(pkt))

	maxPlainPacketSize := cf.cm.c.MaxPacketSize
	if cf.cm.srtpOutCtx != nil {
		maxPlainPacketSize -= srtpOverhead
	}

	buf := make([]byte, maxPlainPacketSize)
	n, err := pkt.MarshalTo(buf)
	if err != nil {
		return err
	}
	buf = buf[:n]

	if cf.cm.srtpOutCtx != nil {
		encr := make([]byte, cf.cm.c.MaxPacketSize)
		encr, err = cf.cm.srtpOutCtx.encryptRTP(encr, buf, &pkt.Header)
		if err != nil {
			return err
		}
		buf = encr
	}

	cf.cm.c.writerMutex.RLock()
	defer cf.cm.c.writerMutex.RUnlock()

	if cf.cm.c.writer == nil {
		return nil
	}

	ok := cf.cm.c.writer.Push(func() error {
		return cf.writePacketRTPInQueue(buf)
	})
	if !ok {
		return liberrors.ErrClientWriteQueueFull{}
	}

	return nil
}

func (cf *clientFormat) writePacketRTPInQueueUDP(payload []byte) error {
	err := cf.cm.udpRTPListener.write(payload)
	if err != nil {
		return err
	}

	atomic.AddUint64(cf.cm.bytesSent, uint64(len(payload)))
	atomic.AddUint64(cf.rtpPacketsSent, 1)
	return nil
}

func (cf *clientFormat) writePacketRTPInQueueTCP(payload []byte) error {
	cf.cm.c.tcpFrame.Channel = cf.cm.tcpChannel
	cf.cm.c.tcpFrame.Payload = payload
	cf.cm.c.nconn.SetWriteDeadline(time.Now().Add(cf.cm.c.WriteTimeout))
	err := cf.cm.c.conn.WriteInterleavedFrame(cf.cm.c.tcpFrame, cf.cm.c.tcpBuffer)
	if err != nil {
		return err
	}

	atomic.AddUint64(cf.cm.bytesSent, uint64(len(payload)))
	atomic.AddUint64(cf.rtpPacketsSent, 1)
	return nil
}
