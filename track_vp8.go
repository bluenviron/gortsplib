package gortsplib

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/pion/rtp"

	"github.com/aler9/gortsplib/pkg/rtpcodecs/rtpvp8"
)

// TrackVP8 is a VP8 track.
type TrackVP8 struct {
	PayloadType uint8
	MaxFR       *int
	MaxFS       *int
}

// String returns a description of the track.
func (t *TrackVP8) String() string {
	return "VP8"
}

// ClockRate returns the clock rate.
func (t *TrackVP8) ClockRate() int {
	return 90000
}

// GetPayloadType returns the payload type.
func (t *TrackVP8) GetPayloadType() uint8 {
	return t.PayloadType
}

func (t *TrackVP8) unmarshal(payloadType uint8, clock string, codec string, rtpmap string, fmtp string) error {
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
			}
		}
	}

	return nil
}

func (t *TrackVP8) marshal() (string, string) {
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

func (t *TrackVP8) clone() Track {
	return &TrackVP8{
		PayloadType: t.PayloadType,
		MaxFR:       t.MaxFR,
		MaxFS:       t.MaxFS,
	}
}

func (t *TrackVP8) ptsEqualsDTS(*rtp.Packet) bool {
	return true
}

// CreateDecoder creates a decoder able to decode the content of the track.
func (t *TrackVP8) CreateDecoder() *rtpvp8.Decoder {
	d := &rtpvp8.Decoder{}
	d.Init()
	return d
}
