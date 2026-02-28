package format //nolint:revive,dupl

import (
	"github.com/pion/rtp"

	"github.com/bluenviron/gortsplib/v5/pkg/format/rtpmpegts"
)

// MPEGTS is the RTP format for MPEG-TS.
// Specification: RFC2250
type MPEGTS struct {
	// in Go, empty structs share the same pointer,
	// therefore they cannot be used as map keys
	// or in equality operations. Prevent this.
	unused int //nolint:unused
}

func (f *MPEGTS) unmarshal(_ *unmarshalContext) error {
	return nil
}

// Codec implements Format.
func (f *MPEGTS) Codec() string {
	return "MPEG-TS"
}

// ClockRate implements Format.
func (f *MPEGTS) ClockRate() int {
	return 90000
}

// PayloadType implements Format.
func (f *MPEGTS) PayloadType() uint8 {
	return 33
}

// RTPMap implements Format.
func (f *MPEGTS) RTPMap() string {
	return "MP2T/90000"
}

// FMTP implements Format.
func (f *MPEGTS) FMTP() map[string]string {
	return nil
}

// PTSEqualsDTS implements Format.
func (f *MPEGTS) PTSEqualsDTS(*rtp.Packet) bool {
	return true
}

// CreateDecoder creates a decoder able to decode the content of the format.
func (f *MPEGTS) CreateDecoder() (*rtpmpegts.Decoder, error) {
	d := &rtpmpegts.Decoder{}

	err := d.Init()
	if err != nil {
		return nil, err
	}

	return d, nil
}

// CreateEncoder creates an encoder able to encode the content of the format.
func (f *MPEGTS) CreateEncoder() (*rtpmpegts.Encoder, error) {
	e := &rtpmpegts.Encoder{}

	err := e.Init()
	if err != nil {
		return nil, err
	}

	return e, nil
}
