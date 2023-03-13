package format

import (
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"

	"github.com/pion/rtp"
)

// Vorbis is a format that uses the Vorbis codec.
type Vorbis struct {
	PayloadTyp    uint8
	SampleRate    int
	ChannelCount  int
	Configuration []byte
}

// String implements Format.
func (t *Vorbis) String() string {
	return "Vorbis"
}

// ClockRate implements Format.
func (t *Vorbis) ClockRate() int {
	return t.SampleRate
}

// PayloadType implements Format.
func (t *Vorbis) PayloadType() uint8 {
	return t.PayloadTyp
}

func (t *Vorbis) unmarshal(payloadType uint8, clock string, codec string, rtpmap string, fmtp map[string]string) error {
	t.PayloadTyp = payloadType

	tmp := strings.SplitN(clock, "/", 2)
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

	for key, val := range fmtp {
		if key == "configuration" {
			conf, err := base64.StdEncoding.DecodeString(val)
			if err != nil {
				return fmt.Errorf("invalid AAC config (%v)", val)
			}

			t.Configuration = conf
		}
	}

	if t.Configuration == nil {
		return fmt.Errorf("config is missing")
	}

	return nil
}

// Marshal implements Format.
func (t *Vorbis) Marshal() (string, map[string]string) {
	fmtp := map[string]string{
		"configuration": base64.StdEncoding.EncodeToString(t.Configuration),
	}

	return "VORBIS/" + strconv.FormatInt(int64(t.SampleRate), 10) +
			"/" + strconv.FormatInt(int64(t.ChannelCount), 10),
		fmtp
}

// PTSEqualsDTS implements Format.
func (t *Vorbis) PTSEqualsDTS(*rtp.Packet) bool {
	return true
}
