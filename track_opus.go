package gortsplib

import (
	"fmt"
	"strconv"
	"strings"

	psdp "github.com/pion/sdp/v3"

	"github.com/aler9/gortsplib/pkg/base"
)

// TrackOpus is a Opus track.
type TrackOpus struct {
	control      string
	payloadType  uint8
	sampleRate   int
	channelCount int
}

// NewTrackOpus allocates a TrackOpus.
func NewTrackOpus(payloadType uint8, sampleRate int, channelCount int) (*TrackOpus, error) {
	return &TrackOpus{
		payloadType:  payloadType,
		sampleRate:   sampleRate,
		channelCount: channelCount,
	}, nil
}

func newTrackOpusFromMediaDescription(
	payloadType uint8,
	rtpmap string,
	md *psdp.MediaDescription) (*TrackOpus, error) {
	control := trackFindControl(md)
	tmp := strings.SplitN(rtpmap, "/", 3)
	if len(tmp) != 3 {
		return nil, fmt.Errorf("invalid rtpmap (%v)", rtpmap)
	}

	sampleRate, err := strconv.ParseInt(tmp[1], 10, 64)
	if err != nil {
		return nil, err
	}

	channelCount, err := strconv.ParseInt(tmp[2], 10, 64)
	if err != nil {
		return nil, err
	}

	return &TrackOpus{
		control:      control,
		payloadType:  payloadType,
		sampleRate:   int(sampleRate),
		channelCount: int(channelCount),
	}, nil
}

// ClockRate returns the track clock rate.
func (t *TrackOpus) ClockRate() int {
	return t.sampleRate
}

func (t *TrackOpus) clone() Track {
	return &TrackOpus{
		control:      t.control,
		payloadType:  t.payloadType,
		sampleRate:   t.sampleRate,
		channelCount: t.channelCount,
	}
}

// GetControl returns the track control.
func (t *TrackOpus) GetControl() string {
	return t.control
}

// SetControl sets the track control.
func (t *TrackOpus) SetControl(c string) {
	t.control = c
}

func (t *TrackOpus) url(contentBase *base.URL) (*base.URL, error) {
	return trackURL(t, contentBase)
}

// MediaDescription returns the structured media information from the SDP
func (t *TrackOpus) MediaDescription() *psdp.MediaDescription {
	typ := strconv.FormatInt(int64(t.payloadType), 10)

	return &psdp.MediaDescription{
		MediaName: psdp.MediaName{
			Media:   "audio",
			Protos:  []string{"RTP", "AVP"},
			Formats: []string{typ},
		},
		Attributes: []psdp.Attribute{
			{
				Key: "rtpmap",
				Value: typ + " opus/" + strconv.FormatInt(int64(t.sampleRate), 10) +
					"/" + strconv.FormatInt(int64(t.channelCount), 10),
			},
			{
				Key: "fmtp",
				Value: typ + " sprop-stereo=" + func() string {
					if t.channelCount == 2 {
						return "1"
					}
					return "0"
				}(),
			},
			{
				Key:   "control",
				Value: t.control,
			},
		},
	}
}
