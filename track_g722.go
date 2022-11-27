package gortsplib //nolint:dupl

import (
	"fmt"
	"strings"

	"github.com/pion/rtp"

	"github.com/aler9/gortsplib/pkg/rtpcodecs/rtpsimpleaudio"
)

// TrackG722 is a G722 track.
type TrackG722 struct{}

// String returns a description of the track.
func (t *TrackG722) String() string {
	return "G722"
}

// ClockRate returns the clock rate.
func (t *TrackG722) ClockRate() int {
	return 8000
}

// GetPayloadType returns the payload type.
func (t *TrackG722) GetPayloadType() uint8 {
	return 9
}

func (t *TrackG722) unmarshal(payloadType uint8, clock string, codec string, rtpmap string, fmtp string) error {
	tmp := strings.Split(clock, "/")
	if len(tmp) == 2 && tmp[1] != "1" {
		return fmt.Errorf("G722 tracks can have only one channel")
	}

	return nil
}

func (t *TrackG722) marshal() (string, string) {
	return "G722/8000", ""
}

func (t *TrackG722) clone() Track {
	return &TrackG722{}
}

func (t *TrackG722) ptsEqualsDTS(*rtp.Packet) bool {
	return true
}

// CreateDecoder creates a decoder able to decode the content of the track.
func (t *TrackG722) CreateDecoder() *rtpsimpleaudio.Decoder {
	d := &rtpsimpleaudio.Decoder{
		SampleRate: 8000,
	}
	d.Init()
	return d
}

// CreateEncoder creates an encoder able to encode the content of the track.
func (t *TrackG722) CreateEncoder() *rtpsimpleaudio.Encoder {
	e := &rtpsimpleaudio.Encoder{
		PayloadType: 9,
		SampleRate:  8000,
	}
	e.Init()
	return e
}
