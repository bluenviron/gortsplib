package gortsplib

import (
	"strconv"
	"strings"

	"github.com/pion/rtp"
	psdp "github.com/pion/sdp/v3"
)

// Track is a RTSP track.
type Track interface {
	// String returns a description of the track.
	String() string

	// ClockRate returns the clock rate.
	ClockRate() int

	// GetPayloadType returns the payload type.
	GetPayloadType() uint8

	unmarshal(payloadType uint8, clock string, codec string, rtpmap string, fmtp string) error
	marshal() (string, string)
	clone() Track
	ptsEqualsDTS(*rtp.Packet) bool
}

func getControlAttribute(attributes []psdp.Attribute) string {
	for _, attr := range attributes {
		if attr.Key == "control" {
			return attr.Value
		}
	}
	return ""
}

func getTrackAttribute(attributes []psdp.Attribute, payloadType uint8, key string) string {
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

func newTrackFromMediaDescription(md *psdp.MediaDescription, payloadTypeStr string) (Track, error) {
	tmp, err := strconv.ParseInt(payloadTypeStr, 10, 8)
	if err != nil {
		return nil, err
	}
	payloadType := uint8(tmp)

	rtpMap := getTrackAttribute(md.Attributes, payloadType, "rtpmap")
	codec, clock := getCodecAndClock(rtpMap)
	codec = strings.ToLower(codec)
	fmtp := getTrackAttribute(md.Attributes, payloadType, "fmtp")

	track := func() Track {
		switch {
		case md.MediaName.Media == "video":
			switch {
			case payloadType == 26:
				return &TrackJPEG{}

			case payloadType == 32:
				return &TrackMPEG2Video{}

			case codec == "h264" && clock == "90000":
				return &TrackH264{}

			case codec == "h265" && clock == "90000":
				return &TrackH265{}

			case codec == "vp8" && clock == "90000":
				return &TrackVP8{}

			case codec == "vp9" && clock == "90000":
				return &TrackVP9{}
			}

		case md.MediaName.Media == "audio":
			switch {
			case payloadType == 0, payloadType == 8:
				return &TrackG711{}

			case payloadType == 9:
				return &TrackG722{}

			case payloadType == 14:
				return &TrackMPEG2Audio{}

			case codec == "l8", codec == "l16", codec == "l24":
				return &TrackLPCM{}

			case codec == "mpeg4-generic":
				return &TrackMPEG4Audio{}

			case codec == "vorbis":
				return &TrackVorbis{}

			case codec == "opus":
				return &TrackOpus{}
			}
		}

		return &TrackGeneric{}
	}()

	err = track.unmarshal(payloadType, clock, codec, rtpMap, fmtp)
	if err != nil {
		return nil, err
	}

	return track, nil
}
