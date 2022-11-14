package gortsplib //nolint:dupl

import (
	"fmt"
	"strings"

	psdp "github.com/pion/sdp/v3"
)

// TrackG722 is a G722 track.
type TrackG722 struct {
	trackBase
}

func newTrackG722FromMediaDescription(
	control string,
	rtpmapPart1 string) (*TrackG722, error,
) {
	tmp := strings.Split(rtpmapPart1, "/")
	if len(tmp) == 2 && tmp[1] != "1" {
		return nil, fmt.Errorf("G722 tracks can have only one channel")
	}

	return &TrackG722{
		trackBase: trackBase{
			control: control,
		},
	}, nil
}

// ClockRate returns the track clock rate.
func (t *TrackG722) ClockRate() int {
	return 8000
}

// MediaDescription returns the track media description in SDP format.
func (t *TrackG722) MediaDescription() *psdp.MediaDescription {
	return &psdp.MediaDescription{
		MediaName: psdp.MediaName{
			Media:   "audio",
			Protos:  []string{"RTP", "AVP"},
			Formats: []string{"9"},
		},
		Attributes: []psdp.Attribute{
			{
				Key:   "rtpmap",
				Value: "9 G722/8000",
			},
			{
				Key:   "control",
				Value: t.control,
			},
		},
	}
}

func (t *TrackG722) clone() Track {
	return &TrackG722{
		trackBase: t.trackBase,
	}
}
