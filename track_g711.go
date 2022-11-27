package gortsplib

import (
	"fmt"
	"strings"

	"github.com/pion/rtp"

	"github.com/aler9/gortsplib/pkg/rtpcodecs/rtpsimpleaudio"
)

// TrackG711 is a G711 track, encoded with mu-law or A-law.
type TrackG711 struct {
	// whether to use mu-law. Otherwise, A-law is used.
	MULaw bool
}

// String returns a description of the track.
func (t *TrackG711) String() string {
	return "G711"
}

// ClockRate returns the clock rate.
func (t *TrackG711) ClockRate() int {
	return 8000
}

// GetPayloadType returns the payload type.
func (t *TrackG711) GetPayloadType() uint8 {
	if t.MULaw {
		return 0
	}
	return 8
}

func (t *TrackG711) unmarshal(payloadType uint8, clock string, codec string, rtpmap string, fmtp string) error {
	tmp := strings.Split(clock, "/")
	if len(tmp) == 2 && tmp[1] != "1" {
		return fmt.Errorf("G711 tracks can have only one channel")
	}

	t.MULaw = (payloadType == 0)

	return nil
}

func (t *TrackG711) marshal() (string, string) {
	if t.MULaw {
		return "PCMU/8000", ""
	}
	return "PCMA/8000", ""
}

func (t *TrackG711) clone() Track {
	return &TrackG711{
		MULaw: t.MULaw,
	}
}

func (t *TrackG711) ptsEqualsDTS(*rtp.Packet) bool {
	return true
}

// CreateDecoder creates a decoder able to decode the content of the track.
func (t *TrackG711) CreateDecoder() *rtpsimpleaudio.Decoder {
	d := &rtpsimpleaudio.Decoder{
		SampleRate: 8000,
	}
	d.Init()
	return d
}

// CreateEncoder creates an encoder able to encode the content of the track.
func (t *TrackG711) CreateEncoder() *rtpsimpleaudio.Encoder {
	e := &rtpsimpleaudio.Encoder{
		PayloadType: t.GetPayloadType(),
		SampleRate:  8000,
	}
	e.Init()
	return e
}
