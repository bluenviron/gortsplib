package format

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/pion/rtp"

	"github.com/aler9/gortsplib/v2/pkg/formatdecenc/rtpvp9"
)

// VP9 is a VP9 format.
type VP9 struct {
	PayloadTyp uint8
	MaxFR      *int
	MaxFS      *int
	ProfileID  *int
}

// String implements Format.
func (t *VP9) String() string {
	return "VP9"
}

// ClockRate implements Format.
func (t *VP9) ClockRate() int {
	return 90000
}

// PayloadType implements Format.
func (t *VP9) PayloadType() uint8 {
	return t.PayloadTyp
}

func (t *VP9) unmarshal(payloadType uint8, clock string, codec string, rtpmap string, fmtp string) error {
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

			case "profile-id":
				val, err := strconv.ParseUint(tmp[1], 10, 64)
				if err != nil {
					return fmt.Errorf("invalid profile-id (%v)", tmp[1])
				}
				v2 := int(val)
				t.ProfileID = &v2
			}
		}
	}

	return nil
}

// Marshal implements Format.
func (t *VP9) Marshal() (string, string) {
	var tmp []string
	if t.MaxFR != nil {
		tmp = append(tmp, "max-fr="+strconv.FormatInt(int64(*t.MaxFR), 10))
	}
	if t.MaxFS != nil {
		tmp = append(tmp, "max-fs="+strconv.FormatInt(int64(*t.MaxFS), 10))
	}
	if t.ProfileID != nil {
		tmp = append(tmp, "profile-id="+strconv.FormatInt(int64(*t.ProfileID), 10))
	}
	var fmtp string
	if tmp != nil {
		fmtp = strings.Join(tmp, ";")
	}

	return "VP9/90000", fmtp
}

// Clone implements Format.
func (t *VP9) Clone() Format {
	return &VP9{
		PayloadTyp: t.PayloadTyp,
		MaxFR:      t.MaxFR,
		MaxFS:      t.MaxFS,
		ProfileID:  t.ProfileID,
	}
}

// PTSEqualsDTS implements Format.
func (t *VP9) PTSEqualsDTS(*rtp.Packet) bool {
	return true
}

// CreateDecoder creates a decoder able to decode the content of the format.
func (t *VP9) CreateDecoder() *rtpvp9.Decoder {
	d := &rtpvp9.Decoder{}
	d.Init()
	return d
}
