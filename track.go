package gortsplib

import (
	"fmt"
	"strconv"
	"strings"

	psdp "github.com/pion/sdp/v3"

	"github.com/aler9/gortsplib/pkg/url"
)

// Track is a RTSP track.
type Track interface {
	// ClockRate returns the track clock rate.
	ClockRate() int

	// GetControl returns the track control attribute.
	GetControl() string

	// SetControl sets the track control attribute.
	SetControl(string)

	// MediaDescription returns the track media description in SDP format.
	MediaDescription() *psdp.MediaDescription

	clone() Track
	url(*url.URL) (*url.URL, error)
}

func getControlAttribute(attributes []psdp.Attribute) string {
	for _, attr := range attributes {
		if attr.Key == "control" {
			return attr.Value
		}
	}
	return ""
}

func getRtpmapAttribute(attributes []psdp.Attribute, payloadType uint8) string {
	for _, attr := range attributes {
		if attr.Key == "rtpmap" {
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

func getFmtpAttribute(attributes []psdp.Attribute, payloadType uint8) string {
	for _, attr := range attributes {
		if attr.Key == "fmtp" {
			if parts := strings.SplitN(attr.Value, " ", 2); len(parts) == 2 {
				if tmp, err := strconv.ParseInt(parts[0], 10, 8); err == nil && uint8(tmp) == payloadType {
					return parts[1]
				}
			}
		}
	}
	return ""
}

func getCodecAndClock(attributes []psdp.Attribute, payloadType uint8) (string, string) {
	rtpmap := getRtpmapAttribute(attributes, payloadType)
	if rtpmap == "" {
		return "", ""
	}

	parts2 := strings.SplitN(rtpmap, "/", 2)
	if len(parts2) != 2 {
		return "", ""
	}

	return parts2[0], parts2[1]
}

func newTrackFromMediaDescription(md *psdp.MediaDescription) (Track, error) {
	if len(md.MediaName.Formats) == 0 {
		return nil, fmt.Errorf("no media formats found")
	}

	control := getControlAttribute(md.Attributes)

	if len(md.MediaName.Formats) == 1 {
		tmp, err := strconv.ParseInt(md.MediaName.Formats[0], 10, 8)
		if err != nil {
			return nil, err
		}
		payloadType := uint8(tmp)

		codec, clock := getCodecAndClock(md.Attributes, payloadType)
		codec = strings.ToLower(codec)

		switch {
		case md.MediaName.Media == "video":
			switch {
			case payloadType == 26:
				return newTrackJPEGFromMediaDescription(control)

			case payloadType == 32:
				return newTrackMPEG2VideoFromMediaDescription(control)

			case codec == "h264" && clock == "90000":
				return newTrackH264FromMediaDescription(control, payloadType, md)

			case codec == "h265" && clock == "90000":
				return newTrackH265FromMediaDescription(control, payloadType, md)

			case codec == "vp8" && clock == "90000":
				return newTrackVP8FromMediaDescription(control, payloadType, md)

			case codec == "vp9" && clock == "90000":
				return newTrackVP9FromMediaDescription(control, payloadType, md)
			}

		case md.MediaName.Media == "audio":
			switch {
			case payloadType == 0:
				return newTrackPCMUFromMediaDescription(control, clock)

			case payloadType == 8:
				return newTrackPCMAFromMediaDescription(control, clock)

			case payloadType == 9:
				return newTrackG722FromMediaDescription(control, clock)

			case payloadType == 14:
				return newTrackMPEG2AudioFromMediaDescription(control)

			case codec == "l8", codec == "l16", codec == "l24":
				return newTrackLPCMFromMediaDescription(control, payloadType, codec, clock)

			case codec == "mpeg4-generic":
				return newTrackMPEG4AudioFromMediaDescription(control, payloadType, md)

			case codec == "vorbis":
				return newTrackVorbisFromMediaDescription(control, payloadType, clock, md)

			case codec == "opus":
				return newTrackOpusFromMediaDescription(control, payloadType, clock)
			}
		}
	}

	return newTrackGenericFromMediaDescription(control, md)
}

type trackBase struct {
	control string
}

// GetControl gets the track control attribute.
func (t *trackBase) GetControl() string {
	return t.control
}

// SetControl sets the track control attribute.
func (t *trackBase) SetControl(c string) {
	t.control = c
}

func (t *trackBase) url(contentBase *url.URL) (*url.URL, error) {
	if contentBase == nil {
		return nil, fmt.Errorf("Content-Base header not provided")
	}

	control := t.GetControl()

	// no control attribute, use base URL
	if control == "" {
		return contentBase, nil
	}

	// control attribute contains an absolute path
	if strings.HasPrefix(control, "rtsp://") {
		ur, err := url.Parse(control)
		if err != nil {
			return nil, err
		}

		// copy host and credentials
		ur.Host = contentBase.Host
		ur.User = contentBase.User
		return ur, nil
	}

	// control attribute contains a relative control attribute
	// insert the control attribute at the end of the URL
	// if there's a query, insert it after the query
	// otherwise insert it after the path
	strURL := contentBase.String()
	if control[0] != '?' && !strings.HasSuffix(strURL, "/") {
		strURL += "/"
	}

	ur, _ := url.Parse(strURL + control)
	return ur, nil
}
