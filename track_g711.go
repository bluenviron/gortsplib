package gortsplib

import (
	"fmt"
	"strings"

	psdp "github.com/pion/sdp/v3"

	"github.com/aler9/gortsplib/pkg/rtpcodecs/rtpsimpleaudio"
)

// TrackG711 is a PCMA track.
type TrackG711 struct {
	// whether to use mu-law. Otherwise, A-law is used.
	MULaw bool

	trackBase
}

func newTrackG711FromMediaDescription(
	control string,
	payloadType uint8,
	clock string,
) (*TrackG711, error,
) {
	tmp := strings.Split(clock, "/")
	if len(tmp) == 2 && tmp[1] != "1" {
		return nil, fmt.Errorf("G711 tracks can have only one channel")
	}

	return &TrackG711{
		MULaw: (payloadType == 0),
		trackBase: trackBase{
			control: control,
		},
	}, nil
}

// String returns the track codec.
func (t *TrackG711) String() string {
	return "G711"
}

// ClockRate returns the track clock rate.
func (t *TrackG711) ClockRate() int {
	return 8000
}

// MediaDescription returns the track media description in SDP format.
func (t *TrackG711) MediaDescription() *psdp.MediaDescription {
	var formats []string
	var rtpmap string
	if t.MULaw {
		formats = []string{"0"}
		rtpmap = "0 PCMU/8000"
	} else {
		formats = []string{"8"}
		rtpmap = "8 PCMA/8000"
	}

	return &psdp.MediaDescription{
		MediaName: psdp.MediaName{
			Media:   "audio",
			Protos:  []string{"RTP", "AVP"},
			Formats: formats,
		},
		Attributes: []psdp.Attribute{
			{
				Key:   "rtpmap",
				Value: rtpmap,
			},
			{
				Key:   "control",
				Value: t.control,
			},
		},
	}
}

func (t *TrackG711) clone() Track {
	return &TrackG711{
		MULaw:     t.MULaw,
		trackBase: t.trackBase,
	}
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
	var payloadType uint8
	if t.MULaw {
		payloadType = 0
	} else {
		payloadType = 8
	}

	e := &rtpsimpleaudio.Encoder{
		PayloadType: payloadType,
		SampleRate:  8000,
	}
	e.Init()
	return e
}
