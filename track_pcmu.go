package gortsplib

import (
	"fmt"
	"strings"

	psdp "github.com/pion/sdp/v3"

	"github.com/aler9/gortsplib/pkg/base"
)

// TrackPCMU is a PCMU track.
type TrackPCMU struct {
	control string
}

// NewTrackPCMU allocates a TrackPCMU.
func NewTrackPCMU() *TrackPCMU {
	return &TrackPCMU{}
}

func newTrackPCMUFromMediaDescription(rtpmapPart1 string,
	md *psdp.MediaDescription) (*TrackPCMU, error,
) {
	control := trackFindControl(md)

	tmp := strings.Split(rtpmapPart1, "/")
	if len(tmp) >= 3 && tmp[2] != "1" {
		return nil, fmt.Errorf("PCMU tracks must have only one channel")
	}

	return &TrackPCMU{
		control: control,
	}, nil
}

// ClockRate returns the track clock rate.
func (t *TrackPCMU) ClockRate() int {
	return 8000
}

func (t *TrackPCMU) clone() Track {
	return &TrackPCMU{}
}

// GetControl returns the track control.
func (t *TrackPCMU) GetControl() string {
	return t.control
}

// SetControl sets the track control.
func (t *TrackPCMU) SetControl(c string) {
	t.control = c
}

func (t *TrackPCMU) url(contentBase *base.URL) (*base.URL, error) {
	return trackURL(t, contentBase)
}

// MediaDescription returns the media description in SDP format.
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
