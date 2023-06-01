package formats

import (
	"encoding/base64"
	"fmt"
	"strconv"
	"sync"

	"github.com/pion/rtp"

	"github.com/bluenviron/gortsplib/v3/pkg/formats/rtph265"
)

// H265 is a RTP format that uses the H265 codec.
// Specification: https://datatracker.ietf.org/doc/html/rfc7798
type H265 struct {
	PayloadTyp uint8
	VPS        []byte
	SPS        []byte
	PPS        []byte
	MaxDONDiff int

	mutex sync.RWMutex
}

func (f *H265) unmarshal(payloadType uint8, _ string, _ string, _ string, fmtp map[string]string) error {
	f.PayloadTyp = payloadType

	for key, val := range fmtp {
		switch key {
		case "sprop-vps":
			var err error
			f.VPS, err = base64.StdEncoding.DecodeString(val)
			if err != nil {
				return fmt.Errorf("invalid sprop-vps (%v)", fmtp)
			}

		case "sprop-sps":
			var err error
			f.SPS, err = base64.StdEncoding.DecodeString(val)
			if err != nil {
				return fmt.Errorf("invalid sprop-sps (%v)", fmtp)
			}

		case "sprop-pps":
			var err error
			f.PPS, err = base64.StdEncoding.DecodeString(val)
			if err != nil {
				return fmt.Errorf("invalid sprop-pps (%v)", fmtp)
			}

		case "sprop-max-don-diff":
			tmp, err := strconv.ParseUint(val, 10, 31)
			if err != nil {
				return fmt.Errorf("invalid sprop-max-don-diff (%v)", fmtp)
			}
			f.MaxDONDiff = int(tmp)
		}
	}

	return nil
}

// String implements Format.
func (f *H265) String() string {
	return "H265"
}

// ClockRate implements Format.
func (f *H265) ClockRate() int {
	return 90000
}

// PayloadType implements Format.
func (f *H265) PayloadType() uint8 {
	return f.PayloadTyp
}

// RTPMap implements Format.
func (f *H265) RTPMap() string {
	return "H265/90000"
}

// FMTP implements Format.
func (f *H265) FMTP() map[string]string {
	f.mutex.RLock()
	defer f.mutex.RUnlock()

	fmtp := make(map[string]string)
	if f.VPS != nil {
		fmtp["sprop-vps"] = base64.StdEncoding.EncodeToString(f.VPS)
	}
	if f.SPS != nil {
		fmtp["sprop-sps"] = base64.StdEncoding.EncodeToString(f.SPS)
	}
	if f.PPS != nil {
		fmtp["sprop-pps"] = base64.StdEncoding.EncodeToString(f.PPS)
	}
	if f.MaxDONDiff != 0 {
		fmtp["sprop-max-don-diff"] = strconv.FormatInt(int64(f.MaxDONDiff), 10)
	}

	return fmtp
}

// PTSEqualsDTS implements Format.
func (f *H265) PTSEqualsDTS(*rtp.Packet) bool {
	return true
}

// CreateDecoder creates a decoder able to decode the content of the format.
//
// Deprecated: this has been replaced by CreateDecoder2() that can also return an error.
func (f *H265) CreateDecoder() *rtph265.Decoder {
	d, _ := f.CreateDecoder2()
	return d
}

// CreateDecoder2 creates a decoder able to decode the content of the format.
func (f *H265) CreateDecoder2() (*rtph265.Decoder, error) {
	d := &rtph265.Decoder{
		MaxDONDiff: f.MaxDONDiff,
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
func (f *H265) CreateEncoder() *rtph265.Encoder {
	e, _ := f.CreateEncoder2()
	return e
}

// CreateEncoder2 creates an encoder able to encode the content of the format.
func (f *H265) CreateEncoder2() (*rtph265.Encoder, error) {
	e := &rtph265.Encoder{
		PayloadType: f.PayloadTyp,
		MaxDONDiff:  f.MaxDONDiff,
	}

	err := e.Init()
	if err != nil {
		return nil, err
	}

	return e, nil
}

// SafeSetParams sets the codec parameters.
func (f *H265) SafeSetParams(vps []byte, sps []byte, pps []byte) {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	f.VPS = vps
	f.SPS = sps
	f.PPS = pps
}

// SafeParams returns the codec parameters.
func (f *H265) SafeParams() ([]byte, []byte, []byte) {
	f.mutex.RLock()
	defer f.mutex.RUnlock()
	return f.VPS, f.SPS, f.PPS
}
