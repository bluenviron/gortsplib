package formats

import (
	"fmt"
	"strconv"

	"github.com/pion/rtp"
)

// AV1 is a RTP format that uses the AV1 codec.
// Specification: https://aomediacodec.github.io/av1-rtp-spec/
type AV1 struct {
	PayloadTyp uint8
	LevelIdx   *int
	Profile    *int
	Tier       *int
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

func (f *AV1) unmarshal(payloadType uint8, clock string, codec string, rtpmap string, fmtp map[string]string) error {
	f.PayloadTyp = payloadType

	for key, val := range fmtp {
		switch key {
		case "level-idx":
			n, err := strconv.ParseUint(val, 10, 64)
			if err != nil {
				return fmt.Errorf("invalid level-idx: %v", val)
			}

			v2 := int(n)
			f.LevelIdx = &v2

		case "profile":
			n, err := strconv.ParseUint(val, 10, 64)
			if err != nil {
				return fmt.Errorf("invalid profile: %v", val)
			}

			v2 := int(n)
			f.Profile = &v2

		case "tier":
			n, err := strconv.ParseUint(val, 10, 64)
			if err != nil {
				return fmt.Errorf("invalid tier: %v", val)
			}

			v2 := int(n)
			f.Tier = &v2
		}
	}

	return nil
}

// Marshal implements Format.
func (f *AV1) Marshal() (string, map[string]string) {
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

	return "AV1/90000", fmtp
}

// PTSEqualsDTS implements Format.
func (f *AV1) PTSEqualsDTS(*rtp.Packet) bool {
	return true
}
