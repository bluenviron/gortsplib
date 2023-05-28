package formats

import (
	"encoding/hex"
	"fmt"
	"strconv"

	"github.com/bluenviron/mediacommon/pkg/codecs/mpeg4audio"
	"github.com/pion/rtp"
)

// MPEG4AudioLATM is a RTP format that uses a MPEG-4 Audio codec.
// Specification: https://datatracker.ietf.org/doc/html/rfc6416#section-7.3
type MPEG4AudioLATM struct {
	PayloadTyp     uint8
	ProfileLevelID int
	Bitrate        *int
	CPresent       *bool
	Config         *mpeg4audio.StreamMuxConfig
	SBREnabled     *bool
}

func (f *MPEG4AudioLATM) unmarshal(
	payloadType uint8, _ string, _ string,
	_ string, fmtp map[string]string,
) error {
	f.PayloadTyp = payloadType
	f.ProfileLevelID = 30 // default value defined by specification

	for key, val := range fmtp {
		switch key {
		case "profile-level-id":
			tmp, err := strconv.ParseUint(val, 10, 31)
			if err != nil {
				return fmt.Errorf("invalid profile-level-id: %v", val)
			}

			f.ProfileLevelID = int(tmp)

		case "bitrate":
			tmp, err := strconv.ParseUint(val, 10, 31)
			if err != nil {
				return fmt.Errorf("invalid bitrate: %v", val)
			}

			v := int(tmp)
			f.Bitrate = &v

		case "cpresent":
			v := (val == "1")
			f.CPresent = &v

		case "config":
			enc, err := hex.DecodeString(val)
			if err != nil {
				return fmt.Errorf("invalid AAC config: %v", val)
			}

			f.Config = &mpeg4audio.StreamMuxConfig{}
			err = f.Config.Unmarshal(enc)
			if err != nil {
				return fmt.Errorf("invalid AAC config: %v", err)
			}

		case "sbr-enabled":
			v := (val == "1")
			f.SBREnabled = &v
		}
	}

	if f.Config == nil {
		return fmt.Errorf("config is missing")
	}

	return nil
}

// String implements Format.
func (f *MPEG4AudioLATM) String() string {
	return "MPEG4-audio-latm"
}

// ClockRate implements Format.
func (f *MPEG4AudioLATM) ClockRate() int {
	return f.Config.Programs[0].Layers[0].AudioSpecificConfig.SampleRate
}

// PayloadType implements Format.
func (f *MPEG4AudioLATM) PayloadType() uint8 {
	return f.PayloadTyp
}

// RTPMap implements Format.
func (f *MPEG4AudioLATM) RTPMap() string {
	aoc := f.Config.Programs[0].Layers[0].AudioSpecificConfig

	sampleRate := aoc.SampleRate
	if aoc.ExtensionSampleRate != 0 {
		sampleRate = aoc.ExtensionSampleRate
	}

	channelCount := aoc.ChannelCount
	if aoc.ExtensionType == mpeg4audio.ObjectTypePS {
		channelCount = 2
	}

	return "MP4A-LATM/" + strconv.FormatInt(int64(sampleRate), 10) +
		"/" + strconv.FormatInt(int64(channelCount), 10)
}

// FMTP implements Format.
func (f *MPEG4AudioLATM) FMTP() map[string]string {
	enc, err := f.Config.Marshal()
	if err != nil {
		return nil
	}

	fmtp := map[string]string{
		"profile-level-id": strconv.FormatInt(int64(f.ProfileLevelID), 10),
		"config":           hex.EncodeToString(enc),
		"object":           strconv.FormatInt(int64(f.Config.Programs[0].Layers[0].AudioSpecificConfig.Type), 10),
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

	return fmtp
}

// PTSEqualsDTS implements Format.
func (f *MPEG4AudioLATM) PTSEqualsDTS(*rtp.Packet) bool {
	return true
}
