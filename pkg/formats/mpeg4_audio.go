package formats

import (
	"encoding/hex"
	"fmt"
	"strconv"

	"github.com/pion/rtp"

	"github.com/bluenviron/gortsplib/v3/pkg/formats/rtpmpeg4audio"
	"github.com/bluenviron/mediacommon/pkg/codecs/mpeg4audio"
)

// MPEG4Audio is a format that uses a MPEG-4 audio codec.
type MPEG4Audio struct {
	PayloadTyp       uint8
	Config           *mpeg4audio.Config
	SizeLength       int
	IndexLength      int
	IndexDeltaLength int
}

// String implements Format.
func (f *MPEG4Audio) String() string {
	return "MPEG4-audio"
}

// ClockRate implements Format.
func (f *MPEG4Audio) ClockRate() int {
	return f.Config.SampleRate
}

// PayloadType implements Format.
func (f *MPEG4Audio) PayloadType() uint8 {
	return f.PayloadTyp
}

func (f *MPEG4Audio) unmarshal(
	payloadType uint8, clock string, codec string,
	rtpmap string, fmtp map[string]string,
) error {
	f.PayloadTyp = payloadType

	for key, val := range fmtp {
		switch key {
		case "config":
			enc, err := hex.DecodeString(val)
			if err != nil {
				return fmt.Errorf("invalid AAC config (%v)", val)
			}

			f.Config = &mpeg4audio.Config{}
			err = f.Config.Unmarshal(enc)
			if err != nil {
				return fmt.Errorf("invalid AAC config (%v)", val)
			}

		case "sizelength":
			n, err := strconv.ParseUint(val, 10, 64)
			if err != nil {
				return fmt.Errorf("invalid AAC SizeLength (%v)", val)
			}
			f.SizeLength = int(n)

		case "indexlength":
			n, err := strconv.ParseUint(val, 10, 64)
			if err != nil {
				return fmt.Errorf("invalid AAC IndexLength (%v)", val)
			}
			f.IndexLength = int(n)

		case "indexdeltalength":
			n, err := strconv.ParseUint(val, 10, 64)
			if err != nil {
				return fmt.Errorf("invalid AAC IndexDeltaLength (%v)", val)
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

// Marshal implements Format.
func (f *MPEG4Audio) Marshal() (string, map[string]string) {
	enc, err := f.Config.Marshal()
	if err != nil {
		return "", nil
	}

	sampleRate := f.Config.SampleRate
	if f.Config.ExtensionSampleRate != 0 {
		sampleRate = f.Config.ExtensionSampleRate
	}

	fmtp := make(map[string]string)

	fmtp["profile-level-id"] = "1"
	fmtp["mode"] = "AAC-hbr"
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

	return "mpeg4-generic/" + strconv.FormatInt(int64(sampleRate), 10) +
		"/" + strconv.FormatInt(int64(f.Config.ChannelCount), 10), fmtp
}

// PTSEqualsDTS implements Format.
func (f *MPEG4Audio) PTSEqualsDTS(*rtp.Packet) bool {
	return true
}

// CreateDecoder creates a decoder able to decode the content of the format.
func (f *MPEG4Audio) CreateDecoder() *rtpmpeg4audio.Decoder {
	d := &rtpmpeg4audio.Decoder{
		SampleRate:       f.Config.SampleRate,
		SizeLength:       f.SizeLength,
		IndexLength:      f.IndexLength,
		IndexDeltaLength: f.IndexDeltaLength,
	}
	d.Init()
	return d
}

// CreateEncoder creates an encoder able to encode the content of the format.
func (f *MPEG4Audio) CreateEncoder() *rtpmpeg4audio.Encoder {
	e := &rtpmpeg4audio.Encoder{
		PayloadType:      f.PayloadTyp,
		SampleRate:       f.Config.SampleRate,
		SizeLength:       f.SizeLength,
		IndexLength:      f.IndexLength,
		IndexDeltaLength: f.IndexDeltaLength,
	}
	e.Init()
	return e
}
