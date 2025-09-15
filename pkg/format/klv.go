package format

import (
	"github.com/pion/rtp"

	"github.com/bluenviron/gortsplib/v5/pkg/format/rtpklv"
)

// KLV is the RTP format for KLV data.
// Specification: RFC6597
type KLV struct {
	PayloadTyp uint8
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
	return "SMPTE336M/90000"
}

// FMTP implements Format.
func (f *KLV) FMTP() map[string]string {
	return nil
}

// PTSEqualsDTS implements Format.
func (f *KLV) PTSEqualsDTS(*rtp.Packet) bool {
	return true
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
