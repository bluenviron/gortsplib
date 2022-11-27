package gortsplib

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/pion/rtp"

	"github.com/aler9/gortsplib/pkg/rtpcodecs/rtpvp9"
)

// TrackVP9 is a VP9 track.
type TrackVP9 struct {
	PayloadType uint8
	MaxFR       *int
	MaxFS       *int
	ProfileID   *int
}

// String returns a description of the track.
func (t *TrackVP9) String() string {
	return "VP9"
}

// ClockRate returns the clock rate.
func (t *TrackVP9) ClockRate() int {
	return 90000
}

// GetPayloadType returns the payload type.
func (t *TrackVP9) GetPayloadType() uint8 {
	return t.PayloadType
}

func (t *TrackVP9) unmarshal(payloadType uint8, clock string, codec string, rtpmap string, fmtp string) error {
	t.PayloadType = payloadType

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

func (t *TrackVP9) marshal() (string, string) {
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

func (t *TrackVP9) clone() Track {
	return &TrackVP9{
		PayloadType: t.PayloadType,
		MaxFR:       t.MaxFR,
		MaxFS:       t.MaxFS,
		ProfileID:   t.ProfileID,
	}
}

func (t *TrackVP9) ptsEqualsDTS(*rtp.Packet) bool {
	return true
}

// CreateDecoder creates a decoder able to decode the content of the track.
func (t *TrackVP9) CreateDecoder() *rtpvp9.Decoder {
	d := &rtpvp9.Decoder{}
	d.Init()
	return d
}
