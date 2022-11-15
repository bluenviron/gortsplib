package gortsplib //nolint:dupl

import (
	"fmt"
	"strings"

	psdp "github.com/pion/sdp/v3"

	"github.com/aler9/gortsplib/pkg/rtpsimpleaudio"
)

// TrackPCMU is a PCMU track.
type TrackPCMU struct {
	trackBase
}

func newTrackPCMUFromMediaDescription(
	control string,
	clock string,
) (*TrackPCMU, error,
) {
	tmp := strings.SplitN(clock, "/", 2)
	if len(tmp) == 2 && tmp[1] != "1" {
		return nil, fmt.Errorf("PCMU tracks can have only one channel")
	}

	return &TrackPCMU{
		trackBase: trackBase{
			control: control,
		},
	}, nil
}

// ClockRate returns the track clock rate.
func (t *TrackPCMU) ClockRate() int {
	return 8000
}

// MediaDescription returns the track media description in SDP format.
func (t *TrackPCMU) MediaDescription() *psdp.MediaDescription {
	return &psdp.MediaDescription{
		MediaName: psdp.MediaName{
			Media:   "audio",
			Protos:  []string{"RTP", "AVP"},
			Formats: []string{"0"},
		},
		Attributes: []psdp.Attribute{
			{
				Key:   "rtpmap",
				Value: "0 PCMU/8000",
			},
			{
				Key:   "control",
				Value: t.control,
			},
		},
	}
}

func (t *TrackPCMU) clone() Track {
	return &TrackPCMU{
		trackBase: t.trackBase,
	}
}

// CreateDecoder creates a decoder able to decode the content of the track.
func (t *TrackPCMU) CreateDecoder() *rtpsimpleaudio.Decoder {
	d := &rtpsimpleaudio.Decoder{
		SampleRate: 8000,
	}
	d.Init()
	return d
}

// CreateEncoder creates an encoder able to encode the content of the track.
func (t *TrackPCMU) CreateEncoder() *rtpsimpleaudio.Encoder {
	e := &rtpsimpleaudio.Encoder{
		PayloadType: 0,
		SampleRate:  8000,
	}
	e.Init()
	return e
}
