// Package rtcpreceiver implements RTCP receiver reports.
package rtcpreceiver

import (
	"math/rand"
	"sync"

	"github.com/pion/rtcp"

	"github.com/aler9/gortsplib/pkg/base"
)

type frameRtpReq struct {
	sequenceNumber uint16
}

type frameRtcpReq struct {
	ssrc          uint32
	ntpTimeMiddle uint32
}

type reportReq struct {
	res chan []byte
}

// RtcpReceiver allows building RTCP receiver reports, by parsing
// incoming frames.
type RtcpReceiver struct {
	mutex                sync.Mutex
	publisherSSRC        uint32
	receiverSSRC         uint32
	sequenceNumberCycles uint16
	lastSequenceNumber   uint16
	lastSenderReport     uint32
}

// New allocates a RtcpReceiver.
func New() *RtcpReceiver {
	return &RtcpReceiver{
		receiverSSRC: rand.Uint32(),
	}
}

// OnFrame processes a RTP or RTCP frame and extract the data needed by RTCP receiver reports.
func (rr *RtcpReceiver) OnFrame(streamType base.StreamType, buf []byte) {
	rr.mutex.Lock()
	defer rr.mutex.Unlock()

	if streamType == base.StreamTypeRtp {
		if len(buf) >= 3 {
			// extract the sequence number of the first frame
			sequenceNumber := uint16(uint16(buf[2])<<8 | uint16(buf[1]))

			if sequenceNumber < rr.lastSequenceNumber {
				rr.sequenceNumberCycles += 1
			}
			rr.lastSequenceNumber = sequenceNumber
		}

	} else {
		// we can afford to unmarshal all RTCP frames
		// since they are sent with a frequency much lower than the one of RTP frames
		frames, err := rtcp.Unmarshal(buf)
		if err == nil {
			for _, frame := range frames {
				if senderReport, ok := (frame).(*rtcp.SenderReport); ok {
					rr.publisherSSRC = senderReport.SSRC
					rr.lastSenderReport = uint32(senderReport.NTPTime >> 16)
				}
			}
		}
	}
}

// Report generates a RTCP receiver report.
func (rr *RtcpReceiver) Report() []byte {
	rr.mutex.Lock()
	defer rr.mutex.Unlock()

	report := &rtcp.ReceiverReport{
		SSRC: rr.receiverSSRC,
		Reports: []rtcp.ReceptionReport{
			{
				SSRC:               rr.publisherSSRC,
				LastSequenceNumber: uint32(rr.sequenceNumberCycles)<<8 | uint32(rr.lastSequenceNumber),
				LastSenderReport:   rr.lastSenderReport,
			},
		},
	}

	byts, _ := report.Marshal()
	return byts
}
