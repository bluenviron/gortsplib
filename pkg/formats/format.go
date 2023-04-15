// Package formats contains RTP format definitions, decoders and encoders.
package formats

import (
	"strings"

	"github.com/pion/rtp"
)

func getCodecAndClock(rtpMap string) (string, string) {
	parts2 := strings.SplitN(rtpMap, "/", 2)
	if len(parts2) != 2 {
		return "", ""
	}

	return strings.ToLower(parts2[0]), parts2[1]
}

// Format is a RTP format of a media.
// It defines a codec and a payload type used to transmit the media.
type Format interface {
	unmarshal(payloadType uint8, clock string, codec string, rtpmap string, fmtp map[string]string) error

	// String returns a description of the format.
	String() string

	// ClockRate returns the clock rate.
	ClockRate() int

	// PayloadType returns the payload type.
	PayloadType() uint8

	// RTPMap returns the rtpmap attribute.
	RTPMap() string

	// FMTP returns the fmtp attribute.
	FMTP() map[string]string

	// PTSEqualsDTS checks whether PTS is equal to DTS in RTP packets.
	PTSEqualsDTS(*rtp.Packet) bool
}

// Unmarshal decodes a format from a media description.
func Unmarshal(mediaType string, payloadType uint8, rtpMap string, fmtp map[string]string) (Format, error) {
	codec, clock := getCodecAndClock(rtpMap)

	format := func() Format {
		switch {
		case mediaType == "video":
			switch {
			case payloadType == 26:
				return &MJPEG{}

			case payloadType == 32:
				return &MPEG2Video{}

			case payloadType == 33:
				return &MPEGTS{}

			case codec == "mp4v-es" && clock == "90000":
				return &MPEG4Video{}

			case codec == "h264" && clock == "90000":
				return &H264{}

			case codec == "h265" && clock == "90000":
				return &H265{}

			case codec == "vp8" && clock == "90000":
				return &VP8{}

			case codec == "vp9" && clock == "90000":
				return &VP9{}

			case codec == "av1" && clock == "90000":
				return &AV1{}
			}

		case mediaType == "audio":
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
				return &MPEG4AudioGeneric{}

			case codec == "mp4a-latm":
				return &MPEG4AudioLATM{}

			case codec == "vorbis":
				return &Vorbis{}

			case codec == "opus":
				return &Opus{}
			}
		}

		return &Generic{}
	}()

	err := format.unmarshal(payloadType, clock, codec, rtpMap, fmtp)
	if err != nil {
		return nil, err
	}

	return format, nil
}
