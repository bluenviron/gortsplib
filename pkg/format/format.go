// Package format contains format definitions.
package format

import (
	"strconv"
	"strings"

	"github.com/pion/rtp"
	psdp "github.com/pion/sdp/v3"
)

func getFormatAttribute(attributes []psdp.Attribute, payloadType uint8, key string) string {
	for _, attr := range attributes {
		if attr.Key == key {
			v := strings.TrimSpace(attr.Value)
			if parts := strings.SplitN(v, " ", 2); len(parts) == 2 {
				if tmp, err := strconv.ParseInt(parts[0], 10, 8); err == nil && uint8(tmp) == payloadType {
					return parts[1]
				}
			}
		}
	}
	return ""
}

func getCodecAndClock(rtpMap string) (string, string) {
	parts2 := strings.SplitN(rtpMap, "/", 2)
	if len(parts2) != 2 {
		return "", ""
	}

	return parts2[0], parts2[1]
}

// Format is a RTSP format.
type Format interface {
	// String returns a description of the format.
	String() string

	// ClockRate returns the clock rate.
	ClockRate() int

	// PayloadType returns the payload type.
	PayloadType() uint8

	unmarshal(payloadType uint8, clock string, codec string, rtpmap string, fmtp string) error

	// Marshal encodes the format in SDP format.
	Marshal() (string, string)

	// PTSEqualsDTS checks whether PTS is equal to DTS in the RTP packet.
	PTSEqualsDTS(*rtp.Packet) bool
}

// Unmarshal decodes a format from a media description.
func Unmarshal(md *psdp.MediaDescription, payloadTypeStr string) (Format, error) {
	tmp, err := strconv.ParseInt(payloadTypeStr, 10, 8)
	if err != nil {
		return nil, err
	}
	payloadType := uint8(tmp)

	rtpMap := getFormatAttribute(md.Attributes, payloadType, "rtpmap")
	codec, clock := getCodecAndClock(rtpMap)
	codec = strings.ToLower(codec)
	fmtp := getFormatAttribute(md.Attributes, payloadType, "fmtp")

	format := func() Format {
		switch {
		case md.MediaName.Media == "video":
			switch {
			case payloadType == 26:
				return &MJPEG{}

			case payloadType == 32:
				return &MPEG2Video{}

			case codec == "h264" && clock == "90000":
				return &H264{}

			case codec == "h265" && clock == "90000":
				return &H265{}

			case codec == "vp8" && clock == "90000":
				return &VP8{}

			case codec == "vp9" && clock == "90000":
				return &VP9{}
			}

		case md.MediaName.Media == "audio":
			switch {
			case payloadType == 0, payloadType == 8:
				return &G711{}

			case payloadType == 9:
				return &G722{}

			case payloadType == 14:
				return &MPEG2Audio{}

			case codec == "l8", codec == "l16", codec == "l24":
				return &LPCM{}

			case codec == "mpeg4-generic":
				return &MPEG4Audio{}

			case codec == "vorbis":
				return &Vorbis{}

			case codec == "opus":
				return &Opus{}
			}
		}

		return &Generic{}
	}()

	err = format.unmarshal(payloadType, clock, codec, rtpMap, fmtp)
	if err != nil {
		return nil, err
	}

	return format, nil
}
