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
	payloadType uint8
	maxFR       *int
	maxFS       *int
	profileID   *int
}

// NewTrackVP9 allocates a TrackVP9.
func NewTrackVP9(payloadType uint8, maxFR *int, maxFS *int, profileID *int) *TrackVP9 {
	return &TrackVP9{
		payloadType: payloadType,
		maxFR:       maxFR,
		maxFS:       maxFS,
		profileID:   profileID,
	}
}

func newTrackVP9FromMediaDescription(
	control string,
	payloadType uint8,
	md *psdp.MediaDescription,
) (*TrackVP9, error) {
	t := &TrackVP9{
		trackBase: trackBase{
			control: control,
		},
		payloadType: payloadType,
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
			t.maxFR = &v2

		case "max-fs":
			val, err := strconv.ParseUint(tmp[1], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid max-fs (%v)", tmp[1])
			}
			v2 := int(val)
			t.maxFS = &v2

		case "profile-id":
			val, err := strconv.ParseUint(tmp[1], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid profile-id (%v)", tmp[1])
			}
			v2 := int(val)
			t.profileID = &v2
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
		payloadType: t.payloadType,
		maxFR:       t.maxFR,
		maxFS:       t.maxFS,
		profileID:   t.profileID,
	}
}

// MaxFR returns the track max-fr.
func (t *TrackVP9) MaxFR() *int {
	return t.maxFR
}

// MaxFS returns the track max-fs.
func (t *TrackVP9) MaxFS() *int {
	return t.maxFS
}

// ProfileID returns the track profile-id.
func (t *TrackVP9) ProfileID() *int {
	return t.profileID
}

// MediaDescription returns the track media description in SDP format.
func (t *TrackVP9) MediaDescription() *psdp.MediaDescription {
	typ := strconv.FormatInt(int64(t.payloadType), 10)

	fmtp := typ

	var tmp []string
	if t.maxFR != nil {
		tmp = append(tmp, "max-fr="+strconv.FormatInt(int64(*t.maxFR), 10))
	}
	if t.maxFS != nil {
		tmp = append(tmp, "max-fs="+strconv.FormatInt(int64(*t.maxFS), 10))
	}
	if t.profileID != nil {
		tmp = append(tmp, "profile-id="+strconv.FormatInt(int64(*t.profileID), 10))
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
