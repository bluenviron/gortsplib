package gortsplib

import (
	"slices"
	"sync/atomic"
	"time"

	"github.com/pion/rtcp"
	"github.com/pion/rtp"

	"github.com/bluenviron/gortsplib/v4/pkg/format"
	"github.com/bluenviron/gortsplib/v4/pkg/liberrors"
	"github.com/bluenviron/gortsplib/v4/pkg/rtcpreceiver"
	"github.com/bluenviron/gortsplib/v4/pkg/rtcpsender"
	"github.com/bluenviron/gortsplib/v4/pkg/rtplossdetector"
	"github.com/bluenviron/gortsplib/v4/pkg/rtpreorderer"
)

func clientPickLocalSSRC(cf *clientFormat) (uint32, error) {
	var takenSSRCs []uint32 //nolint:prealloc

	for _, cm := range cf.cm.c.setuppedMedias {
		for _, cf := range cm.formats {
			takenSSRCs = append(takenSSRCs, cf.localSSRC)
		}
	}

	for _, cf := range cf.cm.formats {
		takenSSRCs = append(takenSSRCs, cf.localSSRC)
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

type clientFormat struct {
	cm          *clientMedia
	format      format.Format
	onPacketRTP OnPacketRTPFunc

	localSSRC             uint32
	udpReorderer          *rtpreorderer.Reorderer       // play
	tcpLossDetector       *rtplossdetector.LossDetector // play
	rtcpReceiver          *rtcpreceiver.RTCPReceiver    // play
	rtcpSender            *rtcpsender.RTCPSender        // record or back channel
	writePacketRTPInQueue func([]byte) error
	rtpPacketsReceived    *uint64
	rtpPacketsSent        *uint64
	rtpPacketsLost        *uint64
}

func (cf *clientFormat) initialize() error {
	if cf.cm.c.state == clientStatePreRecord {
		cf.localSSRC = cf.cm.c.announceData[cf.cm.media].formats[cf.format.PayloadType()].localSSRC
	} else {
		var err error
		cf.localSSRC, err = clientPickLocalSSRC(cf)
		if err != nil {
			return err
		}
	}

	cf.rtpPacketsReceived = new(uint64)
	cf.rtpPacketsSent = new(uint64)
	cf.rtpPacketsLost = new(uint64)

	return nil
}

func (cf *clientFormat) start() {
	if cf.cm.udpRTPListener != nil {
		cf.writePacketRTPInQueue = cf.writePacketRTPInQueueUDP
	} else {
		cf.writePacketRTPInQueue = cf.writePacketRTPInQueueTCP
	}

	if cf.cm.c.state == clientStateRecord || cf.cm.media.IsBackChannel {
		cf.rtcpSender = &rtcpsender.RTCPSender{
			ClockRate: cf.format.ClockRate(),
			Period:    cf.cm.c.senderReportPeriod,
			TimeNow:   cf.cm.c.timeNow,
			WritePacketRTCP: func(pkt rtcp.Packet) {
				if !cf.cm.c.DisableRTCPSenderReports {
					cf.cm.c.WritePacketRTCP(cf.cm.media, pkt) //nolint:errcheck
				}
			},
		}
		cf.rtcpSender.Initialize()
	} else {
		if cf.cm.udpRTPListener != nil {
			cf.udpReorderer = &rtpreorderer.Reorderer{}
			cf.udpReorderer.Initialize()
		} else {
			cf.tcpLossDetector = &rtplossdetector.LossDetector{}
		}

		cf.rtcpReceiver = &rtcpreceiver.RTCPReceiver{
			ClockRate: cf.format.ClockRate(),
			LocalSSRC: &cf.localSSRC,
			Period:    cf.cm.c.receiverReportPeriod,
			TimeNow:   cf.cm.c.timeNow,
			WritePacketRTCP: func(pkt rtcp.Packet) {
				if cf.cm.udpRTPListener != nil && cf.cm.udpRTCPListener.writeAddr != nil {
					cf.cm.c.WritePacketRTCP(cf.cm.media, pkt) //nolint:errcheck
				}
			},
		}
		err := cf.rtcpReceiver.Initialize()
		if err != nil {
			panic(err)
		}
	}
}

func (cf *clientFormat) stop() {
	if cf.rtcpReceiver != nil {
		cf.rtcpReceiver.Close()
		cf.rtcpReceiver = nil
	}

	if cf.rtcpSender != nil {
		cf.rtcpSender.Close()
	}
}

func (cf *clientFormat) remoteSSRC() (uint32, bool) {
	if cf.rtcpReceiver != nil {
		stats := cf.rtcpReceiver.Stats()
		if stats != nil {
			return stats.RemoteSSRC, true
		}
	}
	return 0, false
}

func (cf *clientFormat) readPacketRTPUDP(pkt *rtp.Packet) {
	packets, lost := cf.udpReorderer.Process(pkt)
	if lost != 0 {
		cf.handlePacketsLost(uint64(lost))
		// do not return
	}

	now := cf.cm.c.timeNow()

	for _, pkt := range packets {
		cf.handlePacketRTP(pkt, now)
	}
}

func (cf *clientFormat) readPacketRTPTCP(pkt *rtp.Packet) {
	lost := cf.tcpLossDetector.Process(pkt)
	if lost != 0 {
		cf.handlePacketsLost(uint64(lost))
		// do not return
	}

	now := cf.cm.c.timeNow()

	cf.handlePacketRTP(pkt, now)
}

func (cf *clientFormat) handlePacketRTP(pkt *rtp.Packet, now time.Time) {
	err := cf.rtcpReceiver.ProcessPacket(pkt, now, cf.format.PTSEqualsDTS(pkt))
	if err != nil {
		cf.cm.onPacketRTPDecodeError(err)
		return
	}

	atomic.AddUint64(cf.rtpPacketsReceived, 1)

	cf.onPacketRTP(pkt)
}

func (cf *clientFormat) handlePacketsLost(lost uint64) {
	atomic.AddUint64(cf.rtpPacketsLost, lost)
	cf.cm.c.OnPacketsLost(lost)
}

func (cf *clientFormat) writePacketRTP(pkt *rtp.Packet, ntp time.Time) error {
	pkt.SSRC = cf.localSSRC

	cf.rtcpSender.ProcessPacket(pkt, ntp, cf.format.PTSEqualsDTS(pkt))

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

	ok := cf.cm.c.writer.push(func() error {
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
