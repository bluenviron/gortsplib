package formats

import (
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	"github.com/bluenviron/mediacommon/pkg/codecs/mpeg4audio"
	"github.com/pion/rtp"

	"github.com/bluenviron/gortsplib/v3/pkg/formats/rtpmpeg4audio"
)

// MPEG4Audio is an alias for MPEG4AudioGeneric.
type MPEG4Audio = MPEG4AudioGeneric

// MPEG4AudioGeneric is a RTP format for a MPEG-4 Audio codec.
// Specification: https://datatracker.ietf.org/doc/html/rfc3640
type MPEG4AudioGeneric struct {
	PayloadTyp       uint8
	ProfileLevelID   int
	Config           *mpeg4audio.Config
	SizeLength       int
	IndexLength      int
	IndexDeltaLength int
}

func (f *MPEG4AudioGeneric) unmarshal(
	payloadType uint8, _ string, _ string,
	_ string, fmtp map[string]string,
) error {
	f.PayloadTyp = payloadType

	for key, val := range fmtp {
		switch key {
		case "streamtype":
			if val != "5" { // AudioStream in ISO 14496-1
				return fmt.Errorf("streamtype of AAC must be 5")
			}

		case "mode":
			if strings.ToLower(val) != "aac-hbr" {
				return fmt.Errorf("unsupported AAC mode: %v", val)
			}

		case "profile-level-id":
			tmp, err := strconv.ParseUint(val, 10, 31)
			if err != nil {
				return fmt.Errorf("invalid profile-level-id: %v", val)
			}

			f.ProfileLevelID = int(tmp)

		case "config":
			enc, err := hex.DecodeString(val)
			if err != nil {
				return fmt.Errorf("invalid AAC config: %v", val)
			}

			f.Config = &mpeg4audio.Config{}
			err = f.Config.Unmarshal(enc)
			if err != nil {
				return fmt.Errorf("invalid AAC config: %v", val)
			}

		case "sizelength":
			n, err := strconv.ParseUint(val, 10, 31)
			if err != nil || n > 100 {
				return fmt.Errorf("invalid AAC SizeLength: %v", val)
			}
			f.SizeLength = int(n)

		case "indexlength":
			n, err := strconv.ParseUint(val, 10, 31)
			if err != nil || n > 100 {
				return fmt.Errorf("invalid AAC IndexLength: %v", val)
			}
			f.IndexLength = int(n)

		case "indexdeltalength":
			n, err := strconv.ParseUint(val, 10, 31)
			if err != nil || n > 100 {
				return fmt.Errorf("invalid AAC IndexDeltaLength: %v", val)
			}
			f.IndexDeltaLength = int(n)
		}
	}

	if f.Config == nil {
		return fmt.Errorf("config is missing")
	}

	if f.SizeLength == 0 {
		return fmt.Errorf("sizelength is missing")
	}

	return nil
}

// Codec implements Format.
func (f *MPEG4AudioGeneric) Codec() string {
	return "MPEG-4 Audio"
}

// String implements Format.
//
// Deprecated: replaced by Codec().
func (f *MPEG4AudioGeneric) String() string {
	return f.Codec()
}

// ClockRate implements Format.
func (f *MPEG4AudioGeneric) ClockRate() int {
	return f.Config.SampleRate
}

// PayloadType implements Format.
func (f *MPEG4AudioGeneric) PayloadType() uint8 {
	return f.PayloadTyp
}

// RTPMap implements Format.
func (f *MPEG4AudioGeneric) RTPMap() string {
	sampleRate := f.Config.SampleRate
	if f.Config.ExtensionSampleRate != 0 {
		sampleRate = f.Config.ExtensionSampleRate
	}

	channelCount := f.Config.ChannelCount
	if f.Config.ExtensionType == mpeg4audio.ObjectTypePS {
		channelCount = 2
	}

	return "mpeg4-generic/" + strconv.FormatInt(int64(sampleRate), 10) +
		"/" + strconv.FormatInt(int64(channelCount), 10)
}

// FMTP implements Format.
func (f *MPEG4AudioGeneric) FMTP() map[string]string {
	enc, err := f.Config.Marshal()
	if err != nil {
		return nil
	}

	profileLevelID := f.ProfileLevelID
	if profileLevelID == 0 { // support legacy definition which didn't include profile-level-id
		profileLevelID = 1
	}

	fmtp := map[string]string{
		"streamtype":       "5",
		"mode":             "AAC-hbr",
		"profile-level-id": strconv.FormatInt(int64(profileLevelID), 10),
	}

	if f.SizeLength > 0 {
		fmtp["sizelength"] = strconv.FormatInt(int64(f.SizeLength), 10)
	}

	if f.IndexLength > 0 {
		fmtp["indexlength"] = strconv.FormatInt(int64(f.IndexLength), 10)
	}

	if f.IndexDeltaLength > 0 {
		fmtp["indexdeltalength"] = strconv.FormatInt(int64(f.IndexDeltaLength), 10)
	}

	fmtp["config"] = hex.EncodeToString(enc)

	return fmtp
}

// PTSEqualsDTS implements Format.
func (f *MPEG4AudioGeneric) PTSEqualsDTS(*rtp.Packet) bool {
	return true
}

// CreateDecoder creates a decoder able to decode the content of the format.
//
// Deprecated: this has been replaced by CreateDecoder2() that can also return an error.
func (f *MPEG4AudioGeneric) CreateDecoder() *rtpmpeg4audio.Decoder {
	d, _ := f.CreateDecoder2()
	return d
}

// CreateDecoder2 creates a decoder able to decode the content of the format.
func (f *MPEG4AudioGeneric) CreateDecoder2() (*rtpmpeg4audio.Decoder, error) {
	d := &rtpmpeg4audio.Decoder{
		SampleRate:       f.Config.SampleRate,
		SizeLength:       f.SizeLength,
		IndexLength:      f.IndexLength,
		IndexDeltaLength: f.IndexDeltaLength,
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
func (f *MPEG4AudioGeneric) CreateEncoder() *rtpmpeg4audio.Encoder {
	e, _ := f.CreateEncoder2()
	return e
}

// CreateEncoder2 creates an encoder able to encode the content of the format.
func (f *MPEG4AudioGeneric) CreateEncoder2() (*rtpmpeg4audio.Encoder, error) {
	e := &rtpmpeg4audio.Encoder{
		PayloadType:      f.PayloadTyp,
		SampleRate:       f.Config.SampleRate,
		SizeLength:       f.SizeLength,
		IndexLength:      f.IndexLength,
		IndexDeltaLength: f.IndexDeltaLength,
	}

	err := e.Init()
	if err != nil {
		return nil, err
	}

	return e, nil
}
