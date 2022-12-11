package format

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/pion/rtp"

	"github.com/aler9/gortsplib/v2/pkg/rtpcodecs/rtpvp8"
)

// VP8 is a VP8 format.
type VP8 struct {
	PayloadTyp uint8
	MaxFR      *int
	MaxFS      *int
}

// String implements Format.
func (t *VP8) String() string {
	return "VP8"
}

// ClockRate implements Format.
func (t *VP8) ClockRate() int {
	return 90000
}

// PayloadType implements Format.
func (t *VP8) PayloadType() uint8 {
	return t.PayloadTyp
}

func (t *VP8) unmarshal(payloadType uint8, clock string, codec string, rtpmap string, fmtp string) error {
	t.PayloadTyp = payloadType

	if fmtp != "" {
		for _, kv := range strings.Split(fmtp, ";") {
			kv = strings.Trim(kv, " ")

			if len(kv) == 0 {
				continue
			}

			tmp := strings.SplitN(kv, "=", 2)
			if len(tmp) != 2 {
				return fmt.Errorf("invalid fmtp attribute (%v)", fmtp)
			}

			switch tmp[0] {
			case "max-fr":
				val, err := strconv.ParseUint(tmp[1], 10, 64)
				if err != nil {
					return fmt.Errorf("invalid max-fr (%v)", tmp[1])
				}
				v2 := int(val)
				t.MaxFR = &v2

			case "max-fs":
				val, err := strconv.ParseUint(tmp[1], 10, 64)
				if err != nil {
					return fmt.Errorf("invalid max-fs (%v)", tmp[1])
				}
				v2 := int(val)
				t.MaxFS = &v2
			}
		}
	}

	return nil
}

// Marshal implements Format.
func (t *VP8) Marshal() (string, string) {
	var tmp []string
	if t.MaxFR != nil {
		tmp = append(tmp, "max-fr="+strconv.FormatInt(int64(*t.MaxFR), 10))
	}
	if t.MaxFS != nil {
		tmp = append(tmp, "max-fs="+strconv.FormatInt(int64(*t.MaxFS), 10))
	}
	var fmtp string
	if tmp != nil {
		fmtp = strings.Join(tmp, ";")
	}

	return "VP8/90000", fmtp
}

// Clone implements Format.
func (t *VP8) Clone() Format {
	return &VP8{
		PayloadTyp: t.PayloadTyp,
		MaxFR:      t.MaxFR,
		MaxFS:      t.MaxFS,
	}
}

// PTSEqualsDTS implements Format.
func (t *VP8) PTSEqualsDTS(*rtp.Packet) bool {
	return true
}

// CreateDecoder creates a decoder able to decode the content of the format.
func (t *VP8) CreateDecoder() *rtpvp8.Decoder {
	d := &rtpvp8.Decoder{}
	d.Init()
	return d
}
