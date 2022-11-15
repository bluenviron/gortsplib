package gortsplib //nolint:dupl

import (
	"fmt"
	"strings"

	psdp "github.com/pion/sdp/v3"

	"github.com/aler9/gortsplib/pkg/rtpcodecs/rtpsimpleaudio"
)

// TrackPCMA is a PCMA track.
type TrackPCMA struct {
	trackBase
}

func newTrackPCMAFromMediaDescription(
	control string,
	clock string,
) (*TrackPCMA, error,
) {
	tmp := strings.Split(clock, "/")
	if len(tmp) == 2 && tmp[1] != "1" {
		return nil, fmt.Errorf("PCMA tracks can have only one channel")
	}

	return &TrackPCMA{
		trackBase: trackBase{
			control: control,
		},
	}, nil
}

// ClockRate returns the track clock rate.
func (t *TrackPCMA) ClockRate() int {
	return 8000
}

// MediaDescription returns the track media description in SDP format.
func (t *TrackPCMA) MediaDescription() *psdp.MediaDescription {
	return &psdp.MediaDescription{
		MediaName: psdp.MediaName{
			Media:   "audio",
			Protos:  []string{"RTP", "AVP"},
			Formats: []string{"8"},
		},
		Attributes: []psdp.Attribute{
			{
				Key:   "rtpmap",
				Value: "8 PCMA/8000",
			},
			{
				Key:   "control",
				Value: t.control,
			},
		},
	}
}

func (t *TrackPCMA) clone() Track {
	return &TrackPCMA{
		trackBase: t.trackBase,
	}
}

// CreateDecoder creates a decoder able to decode the content of the track.
func (t *TrackPCMA) CreateDecoder() *rtpsimpleaudio.Decoder {
	d := &rtpsimpleaudio.Decoder{
		SampleRate: 8000,
	}
	d.Init()
	return d
}

// CreateEncoder creates an encoder able to encode the content of the track.
func (t *TrackPCMA) CreateEncoder() *rtpsimpleaudio.Encoder {
	e := &rtpsimpleaudio.Encoder{
		PayloadType: 8,
		SampleRate:  8000,
	}
	e.Init()
	return e
}
