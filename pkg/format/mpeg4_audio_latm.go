package format

import (
	"encoding/hex"
	"fmt"
	"strconv"

	"github.com/bluenviron/mediacommon/v2/pkg/codecs/mpeg4audio"
	"github.com/pion/rtp"

	"github.com/bluenviron/gortsplib/v5/pkg/format/rtpfragmented"
)

func allLayersHaveSameTypeRateChannelsExtType(c *mpeg4audio.StreamMuxConfig) bool {
	typ := c.Programs[0].Layers[0].AudioSpecificConfig.Type
	rate := c.Programs[0].Layers[0].AudioSpecificConfig.SampleRate
	channels := c.Programs[0].Layers[0].AudioSpecificConfig.ChannelCount
	extensionType := c.Programs[0].Layers[0].AudioSpecificConfig.ExtensionType

	for i, p := range c.Programs {
		for j, l := range p.Layers {
			if i == 0 && j == 0 {
				continue
			}

			if l.AudioSpecificConfig.Type != typ ||
				l.AudioSpecificConfig.SampleRate != rate ||
				l.AudioSpecificConfig.ChannelCount != channels ||
				l.AudioSpecificConfig.ExtensionType != extensionType {
				return false
			}
		}
	}

	return true
}

// MPEG4AudioLATM is the RTP format for a MPEG-4 Audio codec, LATM-encoded.
// Specification: RFC6416, section 7.3
type MPEG4AudioLATM struct {
	// payload type of packets.
	PayloadTyp uint8

	ProfileLevelID  int
	Bitrate         *int
	CPresent        bool
	StreamMuxConfig *mpeg4audio.StreamMuxConfig
	SBREnabled      *bool
}

func (f *MPEG4AudioLATM) unmarshal(ctx *unmarshalContext) error {
	f.PayloadTyp = ctx.payloadType

	// default value set by specification
	f.ProfileLevelID = 30
	f.CPresent = true

	for key, val := range ctx.fmtp {
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

			f.StreamMuxConfig = &mpeg4audio.StreamMuxConfig{}
			err = f.StreamMuxConfig.Unmarshal(enc)
			if err != nil {
				return fmt.Errorf("invalid AAC config: %w", err)
			}

		case "sbr-enabled":
			v := (val == "1")
			f.SBREnabled = &v
		}
	}

	if f.CPresent {
		if f.StreamMuxConfig != nil {
			return fmt.Errorf("config and cpresent can't be used at the same time")
		}

		if ctx.clock != "90000/1" {
			return fmt.Errorf("when cpresent=1, clock rate must be 90000/1, but is %s", ctx.clock)
		}
	} else {
		if f.StreamMuxConfig == nil {
			return fmt.Errorf("config is missing")
		}

		if !allLayersHaveSameTypeRateChannelsExtType(f.StreamMuxConfig) {
			return fmt.Errorf("all LATM layers must have the same type, rate, channel count, extension type")
		}
	}
	return nil
}

// Codec implements Format.
func (f *MPEG4AudioLATM) Codec() string {
	return "MPEG-4 Audio LATM"
}

// ClockRate implements Format.
func (f *MPEG4AudioLATM) ClockRate() int {
	if f.CPresent {
		return 90000
	}

	return f.StreamMuxConfig.Programs[0].Layers[0].AudioSpecificConfig.SampleRate
}

// PayloadType implements Format.
func (f *MPEG4AudioLATM) PayloadType() uint8 {
	return f.PayloadTyp
}

// RTPMap implements Format.
func (f *MPEG4AudioLATM) RTPMap() string {
	if f.CPresent {
		return "MP4A-LATM/90000/1"
	}

	aoc := f.StreamMuxConfig.Programs[0].Layers[0].AudioSpecificConfig

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
	fmtp := map[string]string{
		"profile-level-id": strconv.FormatInt(int64(f.ProfileLevelID), 10),
	}

	if f.Bitrate != nil {
		fmtp["bitrate"] = strconv.FormatInt(int64(*f.Bitrate), 10)
	}

	if f.CPresent {
		fmtp["cpresent"] = "1"
	} else {
		fmtp["cpresent"] = "0"

		enc, err := f.StreamMuxConfig.Marshal()
		if err != nil {
			return nil
		}

		fmtp["config"] = hex.EncodeToString(enc)
		fmtp["object"] = strconv.FormatInt(int64(f.StreamMuxConfig.Programs[0].Layers[0].AudioSpecificConfig.Type), 10)
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
func (f *MPEG4AudioLATM) CreateDecoder() (*rtpfragmented.Decoder, error) {
	d := &rtpfragmented.Decoder{}

	err := d.Init()
	if err != nil {
		return nil, err
	}

	return d, nil
}

// CreateEncoder creates an encoder able to encode the content of the format.
func (f *MPEG4AudioLATM) CreateEncoder() (*rtpfragmented.Encoder, error) {
	e := &rtpfragmented.Encoder{
		PayloadType: f.PayloadTyp,
	}

	err := e.Init()
	if err != nil {
		return nil, err
	}

	return e, nil
}
