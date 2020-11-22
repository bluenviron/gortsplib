// Package rtcpreceiver implements a utility to generate RTCP receiver reports.
package rtcpreceiver

import (
	"math/rand"
	"sync"

	"github.com/pion/rtcp"

	"github.com/aler9/gortsplib/pkg/base"
)

// RtcpReceiver allows to generate RTCP receiver reports.
type RtcpReceiver struct {
	receiverSSRC         uint32
	mutex                sync.Mutex
	firstRtpReceived     bool
	senderSSRC           uint32
	sequenceNumberCycles uint16
	lastSequenceNumber   uint16
	lastSenderReportTime uint32
	totalLost            uint32
	totalLostSinceRR     uint32
	totalSinceRR         uint32
}

// New allocates a RtcpReceiver.
func New(receiverSSRC *uint32) *RtcpReceiver {
	return &RtcpReceiver{
		receiverSSRC: func() uint32 {
			if receiverSSRC == nil {
				return rand.Uint32()
			}
			return *receiverSSRC
		}(),
	}
}

// OnFrame processes a RTP or RTCP frame and extract the needed data.
func (rr *RtcpReceiver) OnFrame(streamType base.StreamType, buf []byte) {
	rr.mutex.Lock()
	defer rr.mutex.Unlock()

	if streamType == base.StreamTypeRtp {
		if len(buf) >= 3 {
			// extract the sequence number of the first frame
			sequenceNumber := uint16(buf[2])<<8 | uint16(buf[3])

			// first frame
			if !rr.firstRtpReceived {
				rr.firstRtpReceived = true
				rr.totalSinceRR = 1
				rr.lastSequenceNumber = sequenceNumber

				// subsequent frames
			} else {
				diff := (sequenceNumber - rr.lastSequenceNumber)

				if sequenceNumber != (rr.lastSequenceNumber + 1) {
					rr.totalLost += uint32(diff) - 1
					rr.totalLostSinceRR += uint32(diff) - 1
				}

				if sequenceNumber < rr.lastSequenceNumber {
					rr.sequenceNumberCycles += 1
				}

				rr.totalSinceRR += uint32(diff)
				rr.lastSequenceNumber = sequenceNumber
			}
		}

	} else {
		// we can afford to unmarshal all RTCP frames
		// since they are sent with a frequency much lower than the one of RTP frames
		frames, err := rtcp.Unmarshal(buf)
		if err == nil {
			for _, frame := range frames {
				if sr, ok := (frame).(*rtcp.SenderReport); ok {
					rr.senderSSRC = sr.SSRC
					rr.lastSenderReportTime = uint32(sr.NTPTime >> 16)
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
				SSRC:               rr.senderSSRC,
				LastSequenceNumber: uint32(rr.sequenceNumberCycles)<<16 | uint32(rr.lastSequenceNumber),
				LastSenderReport:   rr.lastSenderReportTime,
				FractionLost: func() uint8 {
					f := float64(rr.totalLostSinceRR) / float64(rr.totalSinceRR)

					// equivalent to taking the integer part after multiplying the
					// loss fraction by 256
					return uint8(f * 256)
				}(),
				TotalLost: rr.totalLost,
			},
		},
	}

	rr.totalLostSinceRR = 0
	rr.totalSinceRR = 0

	byts, _ := report.Marshal()
	return byts
}
