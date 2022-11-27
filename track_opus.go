package gortsplib

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/pion/rtp"

	"github.com/aler9/gortsplib/pkg/rtpcodecs/rtpsimpleaudio"
)

// TrackOpus is a Opus track.
type TrackOpus struct {
	PayloadType  uint8
	SampleRate   int
	ChannelCount int
}

// String returns a description of the track.
func (t *TrackOpus) String() string {
	return "Opus"
}

// ClockRate returns the clock rate.
func (t *TrackOpus) ClockRate() int {
	return t.SampleRate
}

// GetPayloadType returns the payload type.
func (t *TrackOpus) GetPayloadType() uint8 {
	return t.PayloadType
}

func (t *TrackOpus) unmarshal(payloadType uint8, clock string, codec string, rtpmap string, fmtp string) error {
	t.PayloadType = payloadType

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

func (t *TrackOpus) marshal() (string, string) {
	fmtp := "sprop-stereo=" + func() string {
		if t.ChannelCount == 2 {
			return "1"
		}
		return "0"
	}()

	return "opus/" + strconv.FormatInt(int64(t.SampleRate), 10) +
		"/" + strconv.FormatInt(int64(t.ChannelCount), 10), fmtp
}

func (t *TrackOpus) clone() Track {
	return &TrackOpus{
		PayloadType:  t.PayloadType,
		SampleRate:   t.SampleRate,
		ChannelCount: t.ChannelCount,
	}
}

func (t *TrackOpus) ptsEqualsDTS(*rtp.Packet) bool {
	return true
}

// CreateDecoder creates a decoder able to decode the content of the track.
func (t *TrackOpus) CreateDecoder() *rtpsimpleaudio.Decoder {
	d := &rtpsimpleaudio.Decoder{
		SampleRate: t.SampleRate,
	}
	d.Init()
	return d
}

// CreateEncoder creates an encoder able to encode the content of the track.
func (t *TrackOpus) CreateEncoder() *rtpsimpleaudio.Encoder {
	e := &rtpsimpleaudio.Encoder{
		PayloadType: t.PayloadType,
		SampleRate:  8000,
	}
	e.Init()
	return e
}
