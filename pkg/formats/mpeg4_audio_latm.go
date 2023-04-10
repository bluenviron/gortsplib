package formats

import (
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	"github.com/pion/rtp"
)

// MPEG4AudioLATM is a RTP format that uses a MPEG-4 audio codec.
// Specification: https://datatracker.ietf.org/doc/html/rfc6416#section-7.3
type MPEG4AudioLATM struct {
	PayloadTyp     uint8
	SampleRate     int
	Channels       int
	ProfileLevelID int
	Bitrate        *int
	Object         int
	CPresent       *bool
	Config         []byte
	SBREnabled     *bool
}

// String implements Format.
func (f *MPEG4AudioLATM) String() string {
	return "MPEG4-audio-latm"
}

// ClockRate implements Format.
func (f *MPEG4AudioLATM) ClockRate() int {
	return f.SampleRate
}

// PayloadType implements Format.
func (f *MPEG4AudioLATM) PayloadType() uint8 {
	return f.PayloadTyp
}

func (f *MPEG4AudioLATM) unmarshal(
	payloadType uint8, clock string, codec string,
	rtpmap string, fmtp map[string]string,
) error {
	tmp := strings.SplitN(clock, "/", 2)
	if len(tmp) != 2 {
		return fmt.Errorf("invalid clock: %v", clock)
	}

	tmp2, err := strconv.ParseInt(tmp[0], 10, 64)
	if err != nil {
		return err
	}
	f.SampleRate = int(tmp2)

	tmp2, err = strconv.ParseInt(tmp[1], 10, 64)
	if err != nil {
		return err
	}
	f.Channels = int(tmp2)

	f.PayloadTyp = payloadType
	f.ProfileLevelID = 30 // default value defined by specification

	for key, val := range fmtp {
		switch key {
		case "profile-level-id":
			tmp, err := strconv.ParseInt(val, 10, 64)
			if err != nil {
				return fmt.Errorf("invalid profile-level-id: %v", val)
			}

			f.ProfileLevelID = int(tmp)

		case "bitrate":
			tmp, err := strconv.ParseInt(val, 10, 64)
			if err != nil {
				return fmt.Errorf("invalid bitrate: %v", val)
			}

			v := int(tmp)
			f.Bitrate = &v

		case "object":
			tmp, err := strconv.ParseInt(val, 10, 64)
			if err != nil {
				return fmt.Errorf("invalid object: %v", val)
			}

			f.Object = int(tmp)

		case "cpresent":
			tmp, err := strconv.ParseInt(val, 10, 64)
			if err != nil {
				return fmt.Errorf("invalid cpresent: %v", val)
			}

			v := (tmp == 1)
			f.CPresent = &v

		case "config":
			var err error
			f.Config, err = hex.DecodeString(val)
			if err != nil {
				return fmt.Errorf("invalid AAC config: %v", val)
			}

		case "sbr-enabled":
			tmp, err := strconv.ParseInt(val, 10, 64)
			if err != nil {
				return fmt.Errorf("invalid SBR-enabled: %v", val)
			}

			v := (tmp == 1)
			f.SBREnabled = &v
		}
	}

	if f.Object == 0 {
		return fmt.Errorf("object is missing")
	}
	if f.Config == nil {
		return fmt.Errorf("config is missing")
	}

	return nil
}

// Marshal implements Format.
func (f *MPEG4AudioLATM) Marshal() (string, map[string]string) {
	fmtp := map[string]string{
		"profile-level-id": strconv.FormatInt(int64(f.ProfileLevelID), 10),
		"config":           hex.EncodeToString(f.Config),
		"object":           strconv.FormatInt(int64(f.Object), 10),
	}

	if f.Bitrate != nil {
		fmtp["bitrate"] = strconv.FormatInt(int64(*f.Bitrate), 10)
	}

	if f.CPresent != nil {
		if *f.CPresent {
			fmtp["cpresent"] = "1"
		} else {
			fmtp["cpresent"] = "0"
		}
	}

	if f.SBREnabled != nil {
		if *f.SBREnabled {
			fmtp["SBR-enabled"] = "1"
		} else {
			fmtp["SBR-enabled"] = "0"
		}
	}

	return "MP4A-LATM/" + strconv.FormatInt(int64(f.SampleRate), 10) +
		"/" + strconv.FormatInt(int64(f.Channels), 10), fmtp
}

// PTSEqualsDTS implements Format.
func (f *MPEG4AudioLATM) PTSEqualsDTS(*rtp.Packet) bool {
	return true
}
