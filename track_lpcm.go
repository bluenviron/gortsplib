package gortsplib

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/pion/rtp"

	"github.com/aler9/gortsplib/pkg/rtpcodecs/rtplpcm"
)

// TrackLPCM is an uncompressed, Linear PCM track.
type TrackLPCM struct {
	PayloadType  uint8
	BitDepth     int
	SampleRate   int
	ChannelCount int
}

// String returns a description of the track.
func (t *TrackLPCM) String() string {
	return "LPCM"
}

// ClockRate returns the clock rate.
func (t *TrackLPCM) ClockRate() int {
	return t.SampleRate
}

// GetPayloadType returns the payload type.
func (t *TrackLPCM) GetPayloadType() uint8 {
	return t.PayloadType
}

func (t *TrackLPCM) unmarshal(payloadType uint8, clock string, codec string, rtpmap string, fmtp string) error {
	t.PayloadType = payloadType

	switch codec {
	case "l8":
		t.BitDepth = 8

	case "l16":
		t.BitDepth = 16

	case "l24":
		t.BitDepth = 24
	}

	tmp := strings.SplitN(clock, "/", 32)
	if len(tmp) != 2 {
		return fmt.Errorf("invalid clock (%v)", clock)
	}

	sampleRate, err := strconv.ParseInt(tmp[0], 10, 64)
	if err != nil {
		return err
	}
	t.SampleRate = int(sampleRate)

	channelCount, err := strconv.ParseInt(tmp[1], 10, 64)
	if err != nil {
		return err
	}
	t.ChannelCount = int(channelCount)

	return nil
}

func (t *TrackLPCM) marshal() (string, string) {
	var codec string
	switch t.BitDepth {
	case 8:
		codec = "L8"

	case 16:
		codec = "L16"

	case 24:
		codec = "L24"
	}

	return codec + "/" + strconv.FormatInt(int64(t.SampleRate), 10) +
		"/" + strconv.FormatInt(int64(t.ChannelCount), 10), ""
}

func (t *TrackLPCM) clone() Track {
	return &TrackLPCM{
		PayloadType:  t.PayloadType,
		BitDepth:     t.BitDepth,
		SampleRate:   t.SampleRate,
		ChannelCount: t.ChannelCount,
	}
}

func (t *TrackLPCM) ptsEqualsDTS(*rtp.Packet) bool {
	return true
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
