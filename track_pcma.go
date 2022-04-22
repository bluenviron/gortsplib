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

// NewTrackPCMA allocates a TrackPCMA.
func NewTrackPCMA() *TrackPCMA {
	return &TrackPCMA{}
}

func newTrackPCMAFromMediaDescription(
	control string,
	rtpmapPart1 string,
	md *psdp.MediaDescription) (*TrackPCMA, error,
) {
	tmp := strings.Split(rtpmapPart1, "/")
	if len(tmp) >= 3 && tmp[2] != "1" {
		return nil, fmt.Errorf("PCMA tracks must have only one channel")
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

func (t *TrackPCMA) clone() Track {
	return &TrackPCMA{
		trackBase: t.trackBase,
	}
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
