package formats

import (
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	"github.com/pion/rtp"
)

// MPEG4Video is a RTP format that uses the video codec defined in MPEG-4 part 2.
// Specification: https://datatracker.ietf.org/doc/html/rfc6416#section-7.1
type MPEG4Video struct {
	PayloadTyp     uint8
	ProfileLevelID int
	Config         []byte
}

// String implements Format.
func (f *MPEG4Video) String() string {
	return "MPEG4-video"
}

// ClockRate implements Format.
func (f *MPEG4Video) ClockRate() int {
	return 90000
}

// PayloadType implements Format.
func (f *MPEG4Video) PayloadType() uint8 {
	return f.PayloadTyp
}

func (f *MPEG4Video) unmarshal(
	payloadType uint8, clock string, codec string,
	rtpmap string, fmtp map[string]string,
) error {
	f.PayloadTyp = payloadType

	// If this parameter is not specified by
	// the procedure, its default value of 1 (Simple Profile/Level 1) is
	// used.
	f.ProfileLevelID = 1

	for key, val := range fmtp {
		switch key {
		case "profile-level-id":
			tmp, err := strconv.ParseInt(val, 10, 64)
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

// Marshal implements Format.
func (f *MPEG4Video) Marshal() (string, map[string]string) {
	fmtp := map[string]string{
		"profile-level-id": strconv.FormatInt(int64(f.ProfileLevelID), 10),
		"config":           strings.ToUpper(hex.EncodeToString(f.Config)),
	}

	return "MP4V-ES/90000", fmtp
}

// PTSEqualsDTS implements Format.
func (f *MPEG4Video) PTSEqualsDTS(*rtp.Packet) bool {
	return true
}
