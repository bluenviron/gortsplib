package formats

import (
	"encoding/hex"
	"fmt"
	"strconv"

	"github.com/bluenviron/mediacommon/pkg/codecs/mpeg4audio"
	"github.com/pion/rtp"

	"github.com/bluenviron/gortsplib/v3/pkg/formats/rtpmpeg4audiolatm"
)

// MPEG4AudioLATM is a RTP format for a MPEG-4 Audio codec.
// Specification: https://datatracker.ietf.org/doc/html/rfc6416#section-7.3
type MPEG4AudioLATM struct {
	PayloadTyp     uint8
	ProfileLevelID int
	Bitrate        *int
	CPresent       bool
	Config         *mpeg4audio.StreamMuxConfig
	SBREnabled     *bool
}

func (f *MPEG4AudioLATM) unmarshal(
	payloadType uint8, _ string, _ string,
	_ string, fmtp map[string]string,
) error {
	f.PayloadTyp = payloadType

	// default value set by specification
	f.ProfileLevelID = 30
	f.CPresent = true

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
			f.CPresent = (val == "1")

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

	if f.CPresent {
		if f.Config != nil {
			return fmt.Errorf("config and cpresent can't be used at the same time")
		}
	} else {
		if f.Config == nil {
			return fmt.Errorf("config is missing")
		}
	}

	return nil
}

// Codec implements Format.
func (f *MPEG4AudioLATM) Codec() string {
	return "MPEG-4 Audio"
}

// String implements Format.
//
// Deprecated: replaced by Codec().
func (f *MPEG4AudioLATM) String() string {
	return f.Codec()
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

	if f.CPresent {
		fmtp["cpresent"] = "1"
	} else {
		fmtp["cpresent"] = "0"
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

// CreateDecoder creates a decoder able to decode the content of the format.
//
// Deprecated: this has been replaced by CreateDecoder2() that can also return an error.
func (f *MPEG4AudioLATM) CreateDecoder() *rtpmpeg4audiolatm.Decoder {
	d, _ := f.CreateDecoder2()
	return d
}

// CreateDecoder2 creates a decoder able to decode the content of the format.
func (f *MPEG4AudioLATM) CreateDecoder2() (*rtpmpeg4audiolatm.Decoder, error) {
	d := &rtpmpeg4audiolatm.Decoder{
		Config: f.Config,
	}

	err := d.Init()
	if err != nil {
		return nil, err
	}

	return d, nil
}

// CreateEncoder creates an encoder able to encode the content of the format.
//
// Deprecated: this has been replaced by CreateEncoder2() that can also return an error.
func (f *MPEG4AudioLATM) CreateEncoder() *rtpmpeg4audiolatm.Encoder {
	e, _ := f.CreateEncoder2()
	return e
}

// CreateEncoder2 creates an encoder able to encode the content of the format.
func (f *MPEG4AudioLATM) CreateEncoder2() (*rtpmpeg4audiolatm.Encoder, error) {
	e := &rtpmpeg4audiolatm.Encoder{
		PayloadType: f.PayloadTyp,
		Config:      f.Config,
	}

	err := e.Init()
	if err != nil {
		return nil, err
	}

	return e, nil
}
