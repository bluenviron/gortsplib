package gortsplib

import (
	"fmt"
	"strconv"
	"strings"

	psdp "github.com/pion/sdp/v3"

	"github.com/aler9/gortsplib/pkg/rtpcodecs/rtplpcm"
)

// TrackLPCM is an uncompressed, Linear PCM track.
type TrackLPCM struct {
	PayloadType  uint8
	BitDepth     int
	SampleRate   int
	ChannelCount int

	trackBase
}

func newTrackLPCMFromMediaDescription(
	control string,
	payloadType uint8,
	codec string,
	clock string,
) (*TrackLPCM, error,
) {
	var bitDepth int
	switch codec {
	case "L8":
		bitDepth = 8

	case "L16":
		bitDepth = 16

	case "L24":
		bitDepth = 24
	}

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

	return &TrackLPCM{
		PayloadType:  payloadType,
		BitDepth:     bitDepth,
		SampleRate:   int(sampleRate),
		ChannelCount: int(channelCount),
		trackBase: trackBase{
			control: control,
		},
	}, nil
}

// ClockRate returns the track clock rate.
func (t *TrackLPCM) ClockRate() int {
	return t.SampleRate
}

// MediaDescription returns the track media description in SDP format.
func (t *TrackLPCM) MediaDescription() *psdp.MediaDescription {
	typ := strconv.FormatInt(int64(t.PayloadType), 10)

	var codec string
	switch t.BitDepth {
	case 8:
		codec = "L8"

	case 16:
		codec = "L16"

	case 24:
		codec = "L24"
	}

	return &psdp.MediaDescription{
		MediaName: psdp.MediaName{
			Media:   "audio",
			Protos:  []string{"RTP", "AVP"},
			Formats: []string{typ},
		},
		Attributes: []psdp.Attribute{
			{
				Key: "rtpmap",
				Value: typ + " " + codec + "/" + strconv.FormatInt(int64(t.SampleRate), 10) +
					"/" + strconv.FormatInt(int64(t.ChannelCount), 10),
			},
			{
				Key:   "control",
				Value: t.control,
			},
		},
	}
}

func (t *TrackLPCM) clone() Track {
	return &TrackLPCM{
		PayloadType:  t.PayloadType,
		BitDepth:     t.BitDepth,
		SampleRate:   t.SampleRate,
		ChannelCount: t.ChannelCount,
		trackBase:    t.trackBase,
	}
}

// CreateDecoder creates a decoder able to decode the content of the track.
func (t *TrackLPCM) CreateDecoder() *rtplpcm.Decoder {
	d := &rtplpcm.Decoder{
		BitDepth:     t.BitDepth,
		SampleRate:   t.SampleRate,
		ChannelCount: t.ChannelCount,
	}
	d.Init()
	return d
}

// CreateEncoder creates an encoder able to encode the content of the track.
func (t *TrackLPCM) CreateEncoder() *rtplpcm.Encoder {
	e := &rtplpcm.Encoder{
		PayloadType:  t.PayloadType,
		BitDepth:     t.BitDepth,
		SampleRate:   t.SampleRate,
		ChannelCount: t.ChannelCount,
	}
	e.Init()
	return e
}
