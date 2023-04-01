package formats

import (
	"encoding/hex"
	"fmt"
	"strconv"

	"github.com/pion/rtp"

	"github.com/bluenviron/gortsplib/v3/pkg/codecs/mpeg4audio"
	"github.com/bluenviron/gortsplib/v3/pkg/formats/rtpmpeg4audio"
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
func (t *MPEG4Audio) String() string {
	return "MPEG4-audio"
}

// ClockRate implements Format.
func (t *MPEG4Audio) ClockRate() int {
	return t.Config.SampleRate
}

// PayloadType implements Format.
func (t *MPEG4Audio) PayloadType() uint8 {
	return t.PayloadTyp
}

func (t *MPEG4Audio) unmarshal(
	payloadType uint8, clock string, codec string,
	rtpmap string, fmtp map[string]string,
) error {
	t.PayloadTyp = payloadType

	for key, val := range fmtp {
		switch key {
		case "config":
			enc, err := hex.DecodeString(val)
			if err != nil {
				return fmt.Errorf("invalid AAC config (%v)", val)
			}

			t.Config = &mpeg4audio.Config{}
			err = t.Config.Unmarshal(enc)
			if err != nil {
				return fmt.Errorf("invalid AAC config (%v)", val)
			}

		case "sizelength":
			n, err := strconv.ParseUint(val, 10, 64)
			if err != nil {
				return fmt.Errorf("invalid AAC SizeLength (%v)", val)
			}
			t.SizeLength = int(n)

		case "indexlength":
			n, err := strconv.ParseUint(val, 10, 64)
			if err != nil {
				return fmt.Errorf("invalid AAC IndexLength (%v)", val)
			}
			t.IndexLength = int(n)

		case "indexdeltalength":
			n, err := strconv.ParseUint(val, 10, 64)
			if err != nil {
				return fmt.Errorf("invalid AAC IndexDeltaLength (%v)", val)
			}
			t.IndexDeltaLength = int(n)
		}
	}

	if t.Config == nil {
		return fmt.Errorf("config is missing")
	}

	if t.SizeLength == 0 {
		return fmt.Errorf("sizelength is missing")
	}

	return nil
}

// Marshal implements Format.
func (t *MPEG4Audio) Marshal() (string, map[string]string) {
	enc, err := t.Config.Marshal()
	if err != nil {
		return "", nil
	}

	sampleRate := t.Config.SampleRate
	if t.Config.ExtensionSampleRate != 0 {
		sampleRate = t.Config.ExtensionSampleRate
	}

	fmtp := make(map[string]string)

	fmtp["profile-level-id"] = "1"
	fmtp["mode"] = "AAC-hbr"
	if t.SizeLength > 0 {
		fmtp["sizelength"] = strconv.FormatInt(int64(t.SizeLength), 10)
	}
	if t.IndexLength > 0 {
		fmtp["indexlength"] = strconv.FormatInt(int64(t.IndexLength), 10)
	}
	if t.IndexDeltaLength > 0 {
		fmtp["indexdeltalength"] = strconv.FormatInt(int64(t.IndexDeltaLength), 10)
	}
	fmtp["config"] = hex.EncodeToString(enc)

	return "mpeg4-generic/" + strconv.FormatInt(int64(sampleRate), 10) +
		"/" + strconv.FormatInt(int64(t.Config.ChannelCount), 10), fmtp
}

// PTSEqualsDTS implements Format.
func (t *MPEG4Audio) PTSEqualsDTS(*rtp.Packet) bool {
	return true
}

// CreateDecoder creates a decoder able to decode the content of the format.
func (t *MPEG4Audio) CreateDecoder() *rtpmpeg4audio.Decoder {
	d := &rtpmpeg4audio.Decoder{
		SampleRate:       t.Config.SampleRate,
		SizeLength:       t.SizeLength,
		IndexLength:      t.IndexLength,
		IndexDeltaLength: t.IndexDeltaLength,
	}
	d.Init()
	return d
}

// CreateEncoder creates an encoder able to encode the content of the format.
func (t *MPEG4Audio) CreateEncoder() *rtpmpeg4audio.Encoder {
	e := &rtpmpeg4audio.Encoder{
		PayloadType:      t.PayloadTyp,
		SampleRate:       t.Config.SampleRate,
		SizeLength:       t.SizeLength,
		IndexLength:      t.IndexLength,
		IndexDeltaLength: t.IndexDeltaLength,
	}
	e.Init()
	return e
}
