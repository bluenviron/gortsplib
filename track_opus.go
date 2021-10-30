package gortsplib

import (
	"fmt"
	"strconv"
	"strings"

	psdp "github.com/pion/sdp/v3"
)

// TrackConfigOpus is the configuration of an Opus track.
type TrackConfigOpus struct {
	SampleRate   int
	ChannelCount int
}

// NewTrackOpus initializes an Opus track.
func NewTrackOpus(payloadType uint8, conf *TrackConfigOpus) (*Track, error) {
	typ := strconv.FormatInt(int64(payloadType), 10)

	return &Track{
		Media: &psdp.MediaDescription{
			MediaName: psdp.MediaName{
				Media:   "audio",
				Protos:  []string{"RTP", "AVP"},
				Formats: []string{typ},
			},
			Attributes: []psdp.Attribute{
				{
					Key: "rtpmap",
					Value: typ + " opus/" + strconv.FormatInt(int64(conf.SampleRate), 10) +
						"/" + strconv.FormatInt(int64(conf.ChannelCount), 10),
				},
				{
					Key: "fmtp",
					Value: typ + " sprop-stereo=" + func() string {
						if conf.ChannelCount == 2 {
							return "1"
						}
						return "0"
					}(),
				},
			},
		},
	}, nil
}

// IsOpus checks whether the track is an Opus track.
func (t *Track) IsOpus() bool {
	if t.Media.MediaName.Media != "audio" {
		return false
	}

	v, ok := t.Media.Attribute("rtpmap")
	if !ok {
		return false
	}

	vals := strings.Split(v, " ")
	if len(vals) != 2 {
		return false
	}

	return strings.HasPrefix(vals[1], "opus/")
}

// ExtractConfigOpus extracts the configuration of an Opus track.
func (t *Track) ExtractConfigOpus() (*TrackConfigOpus, error) {
	v, ok := t.Media.Attribute("rtpmap")
	if !ok {
		return nil, fmt.Errorf("rtpmap attribute is missing")
	}

	tmp := strings.SplitN(v, "/", 3)
	if len(tmp) != 3 {
		return nil, fmt.Errorf("invalid rtpmap (%v)", v)
	}

	sampleRate, err := strconv.ParseInt(tmp[1], 10, 64)
	if err != nil {
		return nil, err
	}

	channelCount, err := strconv.ParseInt(tmp[2], 10, 64)
	if err != nil {
		return nil, err
	}

	return &TrackConfigOpus{
		SampleRate:   int(sampleRate),
		ChannelCount: int(channelCount),
	}, nil
}
