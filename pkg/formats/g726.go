package formats

import (
	"strconv"
	"strings"

	"github.com/pion/rtp"
)

// G726 is a RTP format for the G726 codec.
// Specification: https://datatracker.ietf.org/doc/html/rfc3551
type G726 struct {
	PayloadTyp uint8
	BitRate    int
	BigEndian  bool
}

func (f *G726) unmarshal(payloadType uint8, _ string, codec string, _ string, _ map[string]string) error {
	f.PayloadTyp = payloadType

	if strings.HasPrefix(codec, "aal2-") {
		f.BigEndian = true
	}

	switch {
	case strings.HasSuffix(codec, "-16"):
		f.BitRate = 16
	case strings.HasSuffix(codec, "-24"):
		f.BitRate = 24
	case strings.HasSuffix(codec, "-32"):
		f.BitRate = 32
	default:
		f.BitRate = 40
	}

	return nil
}

// Codec implements Format.
func (f *G726) Codec() string {
	return "G726"
}

// String implements Format.
//
// Deprecated: replaced by Codec().
func (f *G726) String() string {
	return f.Codec()
}

// ClockRate implements Format.
func (f *G726) ClockRate() int {
	return 8000
}

// PayloadType implements Format.
func (f *G726) PayloadType() uint8 {
	return f.PayloadTyp
}

// RTPMap implements Format.
func (f *G726) RTPMap() string {
	codec := ""

	if f.BigEndian {
		codec += "AAL2-"
	}

	return codec + "G726-" + strconv.FormatInt(int64(f.BitRate), 10) + "/8000"
}

// FMTP implements Format.
func (f *G726) FMTP() map[string]string {
	return nil
}

// PTSEqualsDTS implements Format.
func (f *G726) PTSEqualsDTS(*rtp.Packet) bool {
	return true
}
