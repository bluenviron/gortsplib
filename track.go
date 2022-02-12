package gortsplib

import (
	"fmt"
	"strconv"
	"strings"

	psdp "github.com/pion/sdp/v3"

	"github.com/aler9/gortsplib/pkg/base"
)

// Track is a RTSP track.
type Track interface {
	// ClockRate returns the track clock rate.
	ClockRate() int
	// GetControl returns the track control.
	GetControl() string
	// SetControl sets the track control.
	SetControl(string)
	// MediaDescription returns the media description in SDP format.
	MediaDescription() *psdp.MediaDescription
	clone() Track
	url(*base.URL) (*base.URL, error)
}

func newTrackFromMediaDescription(md *psdp.MediaDescription) (Track, error) {
	if rtpmap, ok := md.Attribute("rtpmap"); ok {
		rtpmap = strings.TrimSpace(rtpmap)

		if rtpmapParts := strings.Split(rtpmap, " "); len(rtpmapParts) == 2 {
			tmp, err := strconv.ParseInt(rtpmapParts[0], 10, 64)
			if err == nil {
				payloadType := uint8(tmp)

				switch {
				case md.MediaName.Media == "video":
					if rtpmapParts[1] == "H264/90000" {
						return newTrackH264FromMediaDescription(payloadType, md)
					}

				case md.MediaName.Media == "audio":
					switch {
					case strings.HasPrefix(strings.ToLower(rtpmapParts[1]), "mpeg4-generic/"):
						return newTrackAACFromMediaDescription(payloadType, md)

					case strings.HasPrefix(rtpmapParts[1], "opus/"):
						return newTrackOpusFromMediaDescription(payloadType, rtpmapParts[1], md)
					}
				}
			}
		}
	}

	return newTrackGenericFromMediaDescription(md)
}

func trackFindControl(md *psdp.MediaDescription) string {
	for _, attr := range md.Attributes {
		if attr.Key == "control" {
			return attr.Value
		}
	}
	return ""
}

func trackURL(t Track, contentBase *base.URL) (*base.URL, error) {
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
		ur, err := base.ParseURL(control)
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

	ur, _ := base.ParseURL(strURL + control)
	return ur, nil
}
