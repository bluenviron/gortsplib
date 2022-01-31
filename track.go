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
	clone() Track
	getControl() string
	setControl(string)
	url(*base.URL) (*base.URL, error)
	// MediaDescription returns structured SDP media information
	MediaDescription() *psdp.MediaDescription
}

func newTrackFromMediaDescription(md *psdp.MediaDescription) (Track, error) {
	switch md.MediaName.Media {
	case "video":
		if rtpmap, ok := md.Attribute("rtpmap"); ok {
			rtpmap = strings.TrimSpace(rtpmap)

			if vals := strings.Split(rtpmap, " "); len(vals) == 2 && vals[1] == "H264/90000" {
				tmp, err := strconv.ParseInt(vals[0], 10, 64)
				if err != nil {
					return nil, fmt.Errorf("invalid payload type '%s'", vals[0])
				}
				payloadType := uint8(tmp)

				return newTrackH264FromMediaDescription(payloadType, md)
			}
		}

	case "audio":
		if rtpmap, ok := md.Attribute("rtpmap"); ok {
			if vals := strings.Split(rtpmap, " "); len(vals) == 2 {
				tmp, err := strconv.ParseInt(vals[0], 10, 64)
				if err != nil {
					return nil, fmt.Errorf("invalid payload type '%s'", vals[0])
				}
				payloadType := uint8(tmp)

				if strings.HasPrefix(strings.ToLower(vals[1]), "mpeg4-generic/") {
					return newTrackAACFromMediaDescription(payloadType, md)
				}

				if strings.HasPrefix(vals[1], "opus/") {
					return newTrackOpusFromMediaDescription(payloadType, rtpmap, md)
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
		return nil, fmt.Errorf("no Content-Base header provided")
	}

	control := t.getControl()

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
