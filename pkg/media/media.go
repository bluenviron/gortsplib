// Package media contains the media stream definition.
package media

import (
	"fmt"
	"strconv"
	"strings"

	psdp "github.com/pion/sdp/v3"

	"github.com/aler9/gortsplib/v2/pkg/track"
	"github.com/aler9/gortsplib/v2/pkg/url"
)

func getControlAttribute(attributes []psdp.Attribute) string {
	for _, attr := range attributes {
		if attr.Key == "control" {
			return attr.Value
		}
	}
	return ""
}

// Type is the type of a media stream.
type Type string

// standard media stream types.
const (
	TypeVideo       Type = "video"
	TypeAudio       Type = "audio"
	TypeApplication Type = "application"
)

// Media is a media stream. It contains one or more track.
type Media struct {
	// Media type.
	Type Type

	// Control attribute.
	Control string

	// Tracks contained into the media.
	Tracks []track.Track
}

func (m *Media) unmarshal(md *psdp.MediaDescription) error {
	m.Type = Type(md.MediaName.Media)
	m.Control = getControlAttribute(md.Attributes)
	m.Tracks = nil

	for _, payloadType := range md.MediaName.Formats {
		track, err := track.Unmarshal(md, payloadType)
		if err != nil {
			return err
		}

		m.Tracks = append(m.Tracks, track)
	}

	if m.Tracks == nil {
		return fmt.Errorf("no tracks found")
	}

	return nil
}

// Marshal encodes the media in SDP format.
func (m *Media) Marshal() *psdp.MediaDescription {
	md := &psdp.MediaDescription{
		MediaName: psdp.MediaName{
			Media:  string(m.Type),
			Protos: []string{"RTP", "AVP"},
		},
		Attributes: []psdp.Attribute{
			{
				Key:   "control",
				Value: m.Control,
			},
		},
	}

	for _, trak := range m.Tracks {
		typ := strconv.FormatUint(uint64(trak.PayloadType()), 10)
		md.MediaName.Formats = append(md.MediaName.Formats, typ)

		rtpmap, fmtp := trak.Marshal()

		if rtpmap != "" {
			md.Attributes = append(md.Attributes, psdp.Attribute{
				Key:   "rtpmap",
				Value: typ + " " + rtpmap,
			})
		}

		if fmtp != "" {
			md.Attributes = append(md.Attributes, psdp.Attribute{
				Key:   "fmtp",
				Value: typ + " " + fmtp,
			})
		}
	}

	return md
}

// Clone clones the media.
func (m Media) Clone() *Media {
	ret := &Media{
		Type:    m.Type,
		Control: m.Control,
		Tracks:  make([]track.Track, len(m.Tracks)),
	}

	for i, track := range m.Tracks {
		ret.Tracks[i] = track.Clone()
	}

	return ret
}

// URL returns the media URL.
func (m Media) URL(contentBase *url.URL) (*url.URL, error) {
	if contentBase == nil {
		return nil, fmt.Errorf("Content-Base header not provided")
	}

	// no control attribute, use base URL
	if m.Control == "" {
		return contentBase, nil
	}

	// control attribute contains an absolute path
	if strings.HasPrefix(m.Control, "rtsp://") {
		ur, err := url.Parse(m.Control)
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
	if m.Control[0] != '?' && !strings.HasSuffix(strURL, "/") {
		strURL += "/"
	}

	ur, _ := url.Parse(strURL + m.Control)
	return ur, nil
}
