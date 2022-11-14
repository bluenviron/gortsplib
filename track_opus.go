package gortsplib

import (
	"fmt"
	"strconv"
	"strings"

	psdp "github.com/pion/sdp/v3"

	"github.com/aler9/gortsplib/pkg/rtpopus"
)

// TrackOpus is a Opus track.
type TrackOpus struct {
	PayloadType  uint8
	SampleRate   int
	ChannelCount int

	trackBase
}

func newTrackOpusFromMediaDescription(
	control string,
	payloadType uint8,
	clock string,
	md *psdp.MediaDescription,
) (*TrackOpus, error) {
	tmp := strings.SplitN(clock, "/", 32)
	if len(tmp) != 2 {
		return nil, fmt.Errorf("invalid clock (%v)", clock)
	}

	sampleRate, err := strconv.ParseInt(tmp[0], 10, 64)
	if err != nil {
		return nil, err
	}

	channelCount, err := strconv.ParseInt(tmp[1], 10, 64)
	if err != nil {
		return nil, err
	}

	return &TrackOpus{
		PayloadType:  payloadType,
		SampleRate:   int(sampleRate),
		ChannelCount: int(channelCount),
		trackBase: trackBase{
			control: control,
		},
	}, nil
}

// ClockRate returns the track clock rate.
func (t *TrackOpus) ClockRate() int {
	return t.SampleRate
}

// MediaDescription returns the track media description in SDP format.
func (t *TrackOpus) MediaDescription() *psdp.MediaDescription {
	typ := strconv.FormatInt(int64(t.PayloadType), 10)

	return &psdp.MediaDescription{
		MediaName: psdp.MediaName{
			Media:   "audio",
			Protos:  []string{"RTP", "AVP"},
			Formats: []string{typ},
		},
		Attributes: []psdp.Attribute{
			{
				Key: "rtpmap",
				Value: typ + " opus/" + strconv.FormatInt(int64(t.SampleRate), 10) +
					"/" + strconv.FormatInt(int64(t.ChannelCount), 10),
			},
			{
				Key: "fmtp",
				Value: typ + " sprop-stereo=" + func() string {
					if t.ChannelCount == 2 {
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

func (t *TrackOpus) clone() Track {
	return &TrackOpus{
		PayloadType:  t.PayloadType,
		SampleRate:   t.SampleRate,
		ChannelCount: t.ChannelCount,
		trackBase:    t.trackBase,
	}
}

// CreateDecoder creates a decoder able to decode the content of the track.
func (t *TrackOpus) CreateDecoder() *rtpopus.Decoder {
	d := &rtpopus.Decoder{
		SampleRate: t.SampleRate,
	}
	d.Init()
	return d
}
