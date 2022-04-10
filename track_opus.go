package gortsplib

import (
	"fmt"
	"strconv"
	"strings"

	psdp "github.com/pion/sdp/v3"
)

// TrackOpus is a Opus track.
type TrackOpus struct {
	trackBase
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
	control string,
	payloadType uint8,
	rtpmapPart1 string,
	md *psdp.MediaDescription,
) (*TrackOpus, error) {
	tmp := strings.SplitN(rtpmapPart1, "/", 3)
	if len(tmp) != 3 {
		return nil, fmt.Errorf("invalid rtpmap (%v)", rtpmapPart1)
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
		trackBase: trackBase{
			control: control,
		},
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
		trackBase:    t.trackBase,
		payloadType:  t.payloadType,
		sampleRate:   t.sampleRate,
		channelCount: t.channelCount,
	}
}

// ChannelCount returns the channel count.
func (t *TrackOpus) ChannelCount() int {
	return t.channelCount
}

// MediaDescription returns the track media description in SDP format.
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
