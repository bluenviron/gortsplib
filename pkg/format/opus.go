package format

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/pion/rtp"

	"github.com/aler9/gortsplib/v2/pkg/formatdecenc/rtpsimpleaudio"
)

// Opus is a format that uses the Opus codec.
type Opus struct {
	PayloadTyp uint8
	IsStereo   bool
}

// String implements Format.
func (t *Opus) String() string {
	return "Opus"
}

// ClockRate implements Format.
func (t *Opus) ClockRate() int {
	// RFC7587: the RTP timestamp is incremented with a 48000 Hz
	// clock rate for all modes of Opus and all sampling rates.
	return 48000
}

// PayloadType implements Format.
func (t *Opus) PayloadType() uint8 {
	return t.PayloadTyp
}

func (t *Opus) unmarshal(payloadType uint8, clock string, codec string, rtpmap string, fmtp string) error {
	t.PayloadTyp = payloadType

	tmp := strings.SplitN(clock, "/", 2)
	if len(tmp) != 2 {
		return fmt.Errorf("invalid clock (%v)", clock)
	}

	sampleRate, err := strconv.ParseInt(tmp[0], 10, 64)
	if err != nil {
		return err
	}
	if sampleRate != 48000 {
		return fmt.Errorf("invalid sample rate: %d", sampleRate)
	}

	channelCount, err := strconv.ParseInt(tmp[1], 10, 64)
	if err != nil {
		return err
	}
	if channelCount != 2 {
		return fmt.Errorf("invalid channel count: %d", channelCount)
	}

	if fmtp != "" {
		for _, kv := range strings.Split(fmtp, ";") {
			kv = strings.Trim(kv, " ")

			if len(kv) == 0 {
				continue
			}

			tmp := strings.SplitN(kv, "=", 2)
			if len(tmp) != 2 {
				return fmt.Errorf("invalid fmtp (%v)", fmtp)
			}

			if strings.ToLower(tmp[0]) == "sprop-stereo" {
				t.IsStereo = (tmp[1] == "1")
			}
		}
	}

	return nil
}

// Marshal implements Format.
func (t *Opus) Marshal() (string, string) {
	fmtp := "sprop-stereo=" + func() string {
		if t.IsStereo {
			return "1"
		}
		return "0"
	}()

	// RFC7587: The RTP clock rate in "a=rtpmap" MUST be 48000, and the
	// number of channels MUST be 2.
	return "opus/48000/2", fmtp
}

// PTSEqualsDTS implements Format.
func (t *Opus) PTSEqualsDTS(*rtp.Packet) bool {
	return true
}

// CreateDecoder creates a decoder able to decode the content of the format.
func (t *Opus) CreateDecoder() *rtpsimpleaudio.Decoder {
	d := &rtpsimpleaudio.Decoder{
		SampleRate: 48000,
	}
	d.Init()
	return d
}

// CreateEncoder creates an encoder able to encode the content of the format.
func (t *Opus) CreateEncoder() *rtpsimpleaudio.Encoder {
	e := &rtpsimpleaudio.Encoder{
		PayloadType: t.PayloadTyp,
		SampleRate:  48000,
	}
	e.Init()
	return e
}
