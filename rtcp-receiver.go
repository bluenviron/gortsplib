package gortsplib

import (
	"math/rand"
	"time"

	"github.com/pion/rtcp"
)

type rtcpReceiverEvent interface {
	isRtpReceiverEvent()
}

type rtcpReceiverEventFrameRtp struct {
	sequenceNumber uint16
}

func (rtcpReceiverEventFrameRtp) isRtpReceiverEvent() {}

type rtcpReceiverEventFrameRtcp struct {
	ssrc          uint32
	ntpTimeMiddle uint32
}

func (rtcpReceiverEventFrameRtcp) isRtpReceiverEvent() {}

type rtcpReceiverEventLastFrameTime struct {
	res chan time.Time
}

func (rtcpReceiverEventLastFrameTime) isRtpReceiverEvent() {}

type rtcpReceiverEventReport struct {
	res chan []byte
}

func (rtcpReceiverEventReport) isRtpReceiverEvent() {}

type rtcpReceiverEventTerminate struct{}

func (rtcpReceiverEventTerminate) isRtpReceiverEvent() {}

// RtcpReceiver is an object that helps to build RTCP receiver reports from
// incoming frames.
type RtcpReceiver struct {
	events chan rtcpReceiverEvent
	done   chan struct{}
}

// NewRtcpReceiver allocates a RtcpReceiver.
func NewRtcpReceiver() *RtcpReceiver {
	rr := &RtcpReceiver{
		events: make(chan rtcpReceiverEvent),
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
		case rtcpReceiverEventFrameRtp:
			if evt.sequenceNumber < lastSequenceNumber {
				sequenceNumberCycles += 1
			}
			lastSequenceNumber = evt.sequenceNumber
			lastFrameTime = time.Now()

		case rtcpReceiverEventFrameRtcp:
			publisherSSRC = evt.ssrc
			lastSenderReport = evt.ntpTimeMiddle

		case rtcpReceiverEventLastFrameTime:
			evt.res <- lastFrameTime

		case rtcpReceiverEventReport:
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

		case rtcpReceiverEventTerminate:
			break outer
		}
	}

	close(rr.events)

	close(rr.done)
}

// Close closes a RtcpReceiver.
func (rr *RtcpReceiver) Close() {
	rr.events <- rtcpReceiverEventTerminate{}
	<-rr.done
}

// OnFrame process a RTP or RTCP frame and extract the data needed by RTCP receiver reports.
func (rr *RtcpReceiver) OnFrame(streamType StreamType, buf []byte) {
	if streamType == StreamTypeRtp {
		// extract sequence number of first frame
		if len(buf) >= 3 {
			sequenceNumber := uint16(uint16(buf[2])<<8 | uint16(buf[1]))
			rr.events <- rtcpReceiverEventFrameRtp{sequenceNumber}
		}

	} else {
		frames, err := rtcp.Unmarshal(buf)
		if err == nil {
			for _, frame := range frames {
				if senderReport, ok := (frame).(*rtcp.SenderReport); ok {
					rr.events <- rtcpReceiverEventFrameRtcp{
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
	rr.events <- rtcpReceiverEventLastFrameTime{res}
	return <-res
}

// Report generates a RTCP receiver report.
func (rr *RtcpReceiver) Report() []byte {
	res := make(chan []byte)
	rr.events <- rtcpReceiverEventReport{res}
	return <-res
}
