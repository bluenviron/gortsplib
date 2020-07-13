package gortsplib

import (
	"math/rand"
	"time"

	"github.com/pion/rtcp"
)

type RtcpReceiverEvent interface {
	isRtpReceiverEvent()
}

type RtcpReceiverEventFrameRtp struct {
	sequenceNumber uint16
}

func (RtcpReceiverEventFrameRtp) isRtpReceiverEvent() {}

type RtcpReceiverEventFrameRtcp struct {
	ssrc          uint32
	ntpTimeMiddle uint32
}

func (RtcpReceiverEventFrameRtcp) isRtpReceiverEvent() {}

type RtcpReceiverEventLastFrameTime struct {
	res chan time.Time
}

func (RtcpReceiverEventLastFrameTime) isRtpReceiverEvent() {}

type RtcpReceiverEventReport struct {
	res chan []byte
}

func (RtcpReceiverEventReport) isRtpReceiverEvent() {}

type RtcpReceiverEventTerminate struct{}

func (RtcpReceiverEventTerminate) isRtpReceiverEvent() {}

// RtcpReceiver is an object that helps to build RTCP receiver reports from
// incoming frames.
type RtcpReceiver struct {
	events chan RtcpReceiverEvent
	done   chan struct{}
}

// NewRtcpReceiver allocates a RtcpReceiver.
func NewRtcpReceiver() *RtcpReceiver {
	rr := &RtcpReceiver{
		events: make(chan RtcpReceiverEvent),
		done:   make(chan struct{}),
	}
	go rr.run()
	return rr
}

func (rr *RtcpReceiver) run() {
	lastFrameTime := time.Now()
	publisherSSRC := uint32(0)
	receiverSSRC := rand.Uint32()
	sequenceNumberCycles := uint16(0)
	lastSequenceNumber := uint16(0)
	lastSenderReport := uint32(0)

outer:
	for rawEvt := range rr.events {
		switch evt := rawEvt.(type) {
		case RtcpReceiverEventFrameRtp:
			if evt.sequenceNumber < lastSequenceNumber {
				sequenceNumberCycles += 1
			}
			lastSequenceNumber = evt.sequenceNumber
			lastFrameTime = time.Now()

		case RtcpReceiverEventFrameRtcp:
			publisherSSRC = evt.ssrc
			lastSenderReport = evt.ntpTimeMiddle

		case RtcpReceiverEventLastFrameTime:
			evt.res <- lastFrameTime

		case RtcpReceiverEventReport:
			rr := &rtcp.ReceiverReport{
				SSRC: receiverSSRC,
				Reports: []rtcp.ReceptionReport{
					{
						SSRC:               publisherSSRC,
						LastSequenceNumber: uint32(sequenceNumberCycles)<<8 | uint32(lastSequenceNumber),
						LastSenderReport:   lastSenderReport,
					},
				},
			}
			frame, _ := rr.Marshal()
			evt.res <- frame

		case RtcpReceiverEventTerminate:
			break outer
		}
	}

	close(rr.events)

	close(rr.done)
}

// Close closes a RtcpReceiver.
func (rr *RtcpReceiver) Close() {
	rr.events <- RtcpReceiverEventTerminate{}
	<-rr.done
}

// OnFrame process a RTP or RTCP frame and extract the data needed by RTCP receiver reports.
func (rr *RtcpReceiver) OnFrame(streamType StreamType, buf []byte) {
	if streamType == StreamTypeRtp {
		// extract sequence number of first frame
		if len(buf) >= 3 {
			sequenceNumber := uint16(uint16(buf[2])<<8 | uint16(buf[1]))
			rr.events <- RtcpReceiverEventFrameRtp{sequenceNumber}
		}

	} else {
		frames, err := rtcp.Unmarshal(buf)
		if err == nil {
			for _, frame := range frames {
				if senderReport, ok := (frame).(*rtcp.SenderReport); ok {
					rr.events <- RtcpReceiverEventFrameRtcp{
						senderReport.SSRC,
						uint32(senderReport.NTPTime >> 16),
					}
				}
			}
		}
	}
}

// LastFrameTime returns the time the last frame was received.
func (rr *RtcpReceiver) LastFrameTime() time.Time {
	res := make(chan time.Time)
	rr.events <- RtcpReceiverEventLastFrameTime{res}
	return <-res
}

// Report generates a RTCP receiver report.
func (rr *RtcpReceiver) Report() []byte {
	res := make(chan []byte)
	rr.events <- RtcpReceiverEventReport{res}
	return <-res
}
