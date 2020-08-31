package gortsplib

import (
	"math/rand"
	"time"

	"github.com/pion/rtcp"
)

type frameRtpReq struct {
	sequenceNumber uint16
}

type frameRtcpReq struct {
	ssrc          uint32
	ntpTimeMiddle uint32
}

type lastFrameTimeReq struct {
	res chan time.Time
}

type reportReq struct {
	res chan []byte
}

// RtcpReceiver is an object that helps building RTCP receiver reports, by parsing
// incoming frames.
type RtcpReceiver struct {
	frameRtp      chan frameRtpReq
	frameRtcp     chan frameRtcpReq
	lastFrameTime chan lastFrameTimeReq
	report        chan reportReq
	terminate     chan struct{}
	done          chan struct{}
}

// NewRtcpReceiver allocates a RtcpReceiver.
func NewRtcpReceiver() *RtcpReceiver {
	rr := &RtcpReceiver{
		frameRtp:      make(chan frameRtpReq),
		frameRtcp:     make(chan frameRtcpReq),
		lastFrameTime: make(chan lastFrameTimeReq),
		report:        make(chan reportReq),
		terminate:     make(chan struct{}),
		done:          make(chan struct{}),
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
	for {
		select {
		case req := <-rr.frameRtp:
			if req.sequenceNumber < lastSequenceNumber {
				sequenceNumberCycles += 1
			}
			lastSequenceNumber = req.sequenceNumber
			lastFrameTime = time.Now()

		case req := <-rr.frameRtcp:
			publisherSSRC = req.ssrc
			lastSenderReport = req.ntpTimeMiddle

		case req := <-rr.lastFrameTime:
			req.res <- lastFrameTime

		case req := <-rr.report:
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
			req.res <- frame

		case <-rr.terminate:
			break outer
		}
	}

	close(rr.frameRtp)
	close(rr.frameRtcp)
	close(rr.lastFrameTime)
	close(rr.report)
	close(rr.done)
}

// Close closes a RtcpReceiver.
func (rr *RtcpReceiver) Close() {
	close(rr.terminate)
	<-rr.done
}

// OnFrame process a RTP or RTCP frame and extract the data needed by RTCP receiver reports.
func (rr *RtcpReceiver) OnFrame(streamType StreamType, buf []byte) {
	if streamType == StreamTypeRtp {
		// extract sequence number of first frame
		if len(buf) >= 3 {
			sequenceNumber := uint16(uint16(buf[2])<<8 | uint16(buf[1]))
			rr.frameRtp <- frameRtpReq{sequenceNumber}
		}

	} else {
		frames, err := rtcp.Unmarshal(buf)
		if err == nil {
			for _, frame := range frames {
				if senderReport, ok := (frame).(*rtcp.SenderReport); ok {
					rr.frameRtcp <- frameRtcpReq{
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
	rr.lastFrameTime <- lastFrameTimeReq{res}
	return <-res
}

// Report generates a RTCP receiver report.
func (rr *RtcpReceiver) Report() []byte {
	res := make(chan []byte)
	rr.report <- reportReq{res}
	return <-res
}
