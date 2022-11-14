package gortsplib //nolint:dupl

import (
	"fmt"
	"strings"

	psdp "github.com/pion/sdp/v3"
)

// TrackPCMA is a PCMA track.
type TrackPCMA struct {
	trackBase
}

func newTrackPCMAFromMediaDescription(
	control string,
	rtpmapPart1 string) (*TrackPCMA, error,
) {
	tmp := strings.Split(rtpmapPart1, "/")
	if len(tmp) == 2 && tmp[1] != "1" {
		return nil, fmt.Errorf("PCMU tracks can have only one channel")
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
