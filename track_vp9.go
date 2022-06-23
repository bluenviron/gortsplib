package gortsplib

import (
	"fmt"
	"strconv"
	"strings"

	psdp "github.com/pion/sdp/v3"
)

// TrackVP9 is a VP9 track.
type TrackVP9 struct {
	trackBase
	PayloadType uint8
	MaxFR       *int
	MaxFS       *int
	ProfileID   *int
}

func newTrackVP9FromMediaDescription(
	control string,
	payloadType uint8,
	md *psdp.MediaDescription,
) (*TrackVP9, error) {
	t := &TrackVP9{
		PayloadType: payloadType,
		trackBase: trackBase{
			control: control,
		},
	}

	t.fillParamsFromMediaDescription(md)

	return t, nil
}

func (t *TrackVP9) fillParamsFromMediaDescription(md *psdp.MediaDescription) error {
	v, ok := md.Attribute("fmtp")
	if !ok {
		return fmt.Errorf("fmtp attribute is missing")
	}

	tmp := strings.SplitN(v, " ", 2)
	if len(tmp) != 2 {
		return fmt.Errorf("invalid fmtp attribute (%v)", v)
	}

	for _, kv := range strings.Split(tmp[1], ";") {
		kv = strings.Trim(kv, " ")

		if len(kv) == 0 {
			continue
		}

		tmp := strings.SplitN(kv, "=", 2)
		if len(tmp) != 2 {
			return fmt.Errorf("invalid fmtp attribute (%v)", v)
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

	return nil
}

// ClockRate returns the track clock rate.
func (t *TrackVP9) ClockRate() int {
	return 90000
}

func (t *TrackVP9) clone() Track {
	return &TrackVP9{
		trackBase:   t.trackBase,
		PayloadType: t.PayloadType,
		MaxFR:       t.MaxFR,
		MaxFS:       t.MaxFS,
		ProfileID:   t.ProfileID,
	}
}

// MediaDescription returns the track media description in SDP format.
func (t *TrackVP9) MediaDescription() *psdp.MediaDescription {
	typ := strconv.FormatInt(int64(t.PayloadType), 10)

	fmtp := typ

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
	if tmp != nil {
		fmtp += " " + strings.Join(tmp, ";")
	}

	return &psdp.MediaDescription{
		MediaName: psdp.MediaName{
			Media:   "video",
			Protos:  []string{"RTP", "AVP"},
			Formats: []string{typ},
		},
		Attributes: []psdp.Attribute{
			{
				Key:   "rtpmap",
				Value: typ + " VP9/90000",
			},
			{
				Key:   "fmtp",
				Value: fmtp,
			},
			{
				Key:   "control",
				Value: t.control,
			},
		},
	}
}
