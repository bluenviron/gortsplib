package format

import (
	"github.com/pion/rtp"

	"github.com/bluenviron/gortsplib/v4/pkg/format/rtpklv"
	"github.com/bluenviron/mediacommon/v2/pkg/formats/mpegts"
)

// KLV is a RTP format for KLV data, defined in SMPTE-336M.
// Specification: https://datatracker.ietf.org/doc/html/rfc6597
type KLV struct {
	PayloadTyp uint8
	KLVCodec   *mpegts.CodecKLV
}

func (f *KLV) unmarshal(ctx *unmarshalContext) error {
	f.PayloadTyp = ctx.payloadType
	return nil
}

// Codec implements Format.
func (f *KLV) Codec() string {
	return "KLV"
}

// ClockRate implements Format.
func (f *KLV) ClockRate() int {
	return 90000
}

// PayloadType implements Format.
func (f *KLV) PayloadType() uint8 {
	return f.PayloadTyp
}

// RTPMap implements Format.
func (f *KLV) RTPMap() string {
	return "smtpe336m/90000"
}

// FMTP implements Format.
func (f *KLV) FMTP() map[string]string {
	fmtp := make(map[string]string)
	return fmtp
}

// PTSEqualsDTS implements Format.
func (f *KLV) PTSEqualsDTS(*rtp.Packet) bool {
	return false
}

// CreateDecoder creates a decoder able to decode the content of the format.
func (f *KLV) CreateDecoder() (*rtpklv.Decoder, error) {
	d := &rtpklv.Decoder{}

	err := d.Init()
	if err != nil {
		return nil, err
	}

	return d, nil
}

// CreateEncoder creates an encoder able to encode the content of the format.
func (f *KLV) CreateEncoder() (*rtpklv.Encoder, error) {
	e := &rtpklv.Encoder{
		PayloadType: f.PayloadTyp,
	}

	err := e.Init()
	if err != nil {
		return nil, err
	}

	return e, nil
}
