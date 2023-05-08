package formats //nolint:dupl

import (
	"fmt"
	"strconv"

	"github.com/pion/rtp"

	"github.com/bluenviron/gortsplib/v3/pkg/formats/rtpav1"
)

// AV1 is a RTP format that uses the AV1 codec.
// Specification: https://aomediacodec.github.io/av1-rtp-spec/
type AV1 struct {
	PayloadTyp uint8
	LevelIdx   *int
	Profile    *int
	Tier       *int
}

func (f *AV1) unmarshal(payloadType uint8, clock string, codec string, rtpmap string, fmtp map[string]string) error {
	f.PayloadTyp = payloadType

	for key, val := range fmtp {
		switch key {
		case "level-idx":
			n, err := strconv.ParseUint(val, 10, 31)
			if err != nil {
				return fmt.Errorf("invalid level-idx: %v", val)
			}

			v2 := int(n)
			f.LevelIdx = &v2

		case "profile":
			n, err := strconv.ParseUint(val, 10, 31)
			if err != nil {
				return fmt.Errorf("invalid profile: %v", val)
			}

			v2 := int(n)
			f.Profile = &v2

		case "tier":
			n, err := strconv.ParseUint(val, 10, 31)
			if err != nil {
				return fmt.Errorf("invalid tier: %v", val)
			}

			v2 := int(n)
			f.Tier = &v2
		}
	}

	return nil
}

// String implements Format.
func (f *AV1) String() string {
	return "AV1"
}

// ClockRate implements Format.
func (f *AV1) ClockRate() int {
	return 90000
}

// PayloadType implements Format.
func (f *AV1) PayloadType() uint8 {
	return f.PayloadTyp
}

// RTPMap implements Format.
func (f *AV1) RTPMap() string {
	return "AV1/90000"
}

// FMTP implements Format.
func (f *AV1) FMTP() map[string]string {
	fmtp := make(map[string]string)

	if f.LevelIdx != nil {
		fmtp["level-idx"] = strconv.FormatInt(int64(*f.LevelIdx), 10)
	}
	if f.Profile != nil {
		fmtp["profile"] = strconv.FormatInt(int64(*f.Profile), 10)
	}
	if f.Tier != nil {
		fmtp["tier"] = strconv.FormatInt(int64(*f.Tier), 10)
	}

	return fmtp
}

// PTSEqualsDTS implements Format.
func (f *AV1) PTSEqualsDTS(*rtp.Packet) bool {
	return true
}

// CreateDecoder creates a decoder able to decode the content of the format.
func (f *AV1) CreateDecoder() *rtpav1.Decoder {
	d := &rtpav1.Decoder{}
	d.Init()
	return d
}

// CreateEncoder creates an encoder able to encode the content of the format.
func (f *AV1) CreateEncoder() *rtpav1.Encoder {
	e := &rtpav1.Encoder{
		PayloadType: f.PayloadTyp,
	}
	e.Init()
	return e
}
