package gortsplib

import (
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"

	"github.com/pion/rtp"
)

// TrackVorbis is a Vorbis track.
type TrackVorbis struct {
	PayloadType   uint8
	SampleRate    int
	ChannelCount  int
	Configuration []byte
}

// String returns a description of the track.
func (t *TrackVorbis) String() string {
	return "Vorbis"
}

// ClockRate returns the clock rate.
func (t *TrackVorbis) ClockRate() int {
	return t.SampleRate
}

// GetPayloadType returns the payload type.
func (t *TrackVorbis) GetPayloadType() uint8 {
	return t.PayloadType
}

func (t *TrackVorbis) unmarshal(payloadType uint8, clock string, codec string, rtpmap string, fmtp string) error {
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

	if fmtp == "" {
		return fmt.Errorf("fmtp attribute is missing")
	}

	for _, kv := range strings.Split(fmtp, ";") {
		kv = strings.Trim(kv, " ")

		if len(kv) == 0 {
			continue
		}

		tmp := strings.SplitN(kv, "=", 2)
		if len(tmp) != 2 {
			return fmt.Errorf("invalid fmtp (%v)", fmtp)
		}

		if tmp[0] == "configuration" {
			conf, err := base64.StdEncoding.DecodeString(tmp[1])
			if err != nil {
				return fmt.Errorf("invalid AAC config (%v)", tmp[1])
			}

			t.Configuration = conf
		}
	}

	if t.Configuration == nil {
		return fmt.Errorf("config is missing (%v)", fmtp)
	}

	return nil
}

func (t *TrackVorbis) marshal() (string, string) {
	return "VORBIS/" + strconv.FormatInt(int64(t.SampleRate), 10) +
			"/" + strconv.FormatInt(int64(t.ChannelCount), 10),
		"configuration=" + base64.StdEncoding.EncodeToString(t.Configuration)
}

func (t *TrackVorbis) clone() Track {
	return &TrackVorbis{
		PayloadType:   t.PayloadType,
		SampleRate:    t.SampleRate,
		ChannelCount:  t.ChannelCount,
		Configuration: t.Configuration,
	}
}

func (t *TrackVorbis) ptsEqualsDTS(*rtp.Packet) bool {
	return true
}
