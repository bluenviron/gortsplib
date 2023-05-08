package formats

import (
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	"github.com/pion/rtp"

	"github.com/bluenviron/gortsplib/v3/pkg/formats/rtpmpeg4video"
)

// MPEG4Video is an alias for MPEG4VideoES.
type MPEG4Video = MPEG4VideoES

// MPEG4VideoES is a RTP format that uses a MPEG-4 Video codec.
// Specification: https://datatracker.ietf.org/doc/html/rfc6416#section-7.1
type MPEG4VideoES struct {
	PayloadTyp     uint8
	ProfileLevelID int
	Config         []byte
}

func (f *MPEG4VideoES) unmarshal(
	payloadType uint8, clock string, codec string,
	rtpmap string, fmtp map[string]string,
) error {
	f.PayloadTyp = payloadType
	f.ProfileLevelID = 1 // default value defined by specification

	for key, val := range fmtp {
		switch key {
		case "profile-level-id":
			tmp, err := strconv.ParseUint(val, 10, 31)
			if err != nil {
				return fmt.Errorf("invalid profile-level-id: %v", val)
			}

			f.ProfileLevelID = int(tmp)

		case "config":
			var err error
			f.Config, err = hex.DecodeString(val)
			if err != nil {
				return fmt.Errorf("invalid config: %v", val)
			}
		}
	}

	return nil
}

// String implements Format.
func (f *MPEG4VideoES) String() string {
	return "MPEG4-video-es"
}

// ClockRate implements Format.
func (f *MPEG4VideoES) ClockRate() int {
	return 90000
}

// PayloadType implements Format.
func (f *MPEG4VideoES) PayloadType() uint8 {
	return f.PayloadTyp
}

// RTPMap implements Format.
func (f *MPEG4VideoES) RTPMap() string {
	return "MP4V-ES/90000"
}

// FMTP implements Format.
func (f *MPEG4VideoES) FMTP() map[string]string {
	fmtp := map[string]string{
		"profile-level-id": strconv.FormatInt(int64(f.ProfileLevelID), 10),
		"config":           strings.ToUpper(hex.EncodeToString(f.Config)),
	}

	return fmtp
}

// PTSEqualsDTS implements Format.
func (f *MPEG4VideoES) PTSEqualsDTS(*rtp.Packet) bool {
	return true
}

// CreateDecoder creates a decoder able to decode the content of the format.
func (f *MPEG4VideoES) CreateDecoder() *rtpmpeg4video.Decoder {
	d := &rtpmpeg4video.Decoder{}
	d.Init()
	return d
}

// CreateEncoder creates an encoder able to encode the content of the format.
func (f *MPEG4VideoES) CreateEncoder() *rtpmpeg4video.Encoder {
	e := &rtpmpeg4video.Encoder{
		PayloadType: f.PayloadTyp,
	}
	e.Init()
	return e
}
