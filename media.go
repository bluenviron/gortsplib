package gortsplib

import (
	"fmt"
	"strconv"
	"strings"

	psdp "github.com/pion/sdp/v3"

	"github.com/aler9/gortsplib/pkg/sdp"
	"github.com/aler9/gortsplib/pkg/track"
	"github.com/aler9/gortsplib/pkg/url"
)

func getControlAttribute(attributes []psdp.Attribute) string {
	for _, attr := range attributes {
		if attr.Key == "control" {
			return attr.Value
		}
	}
	return ""
}

// MediaType is the type of a media stream.
type MediaType string

// standard media stream types.
const (
	MediaTypeVideo       MediaType = "video"
	MediaTypeAudio       MediaType = "audio"
	MediaTypeApplication MediaType = "application"
)

// Media is a media stream. It contains one or more track.
type Media struct {
	// Media type.
	Type MediaType

	// Control attribute.
	Control string

	// Tracks contained into the media.
	Tracks []track.Track
}

func (m *Media) unmarshal(md *psdp.MediaDescription) error {
	m.Type = MediaType(md.MediaName.Media)
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

func (m *Media) marshal() *psdp.MediaDescription {
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

	for _, track := range m.Tracks {
		typ := strconv.FormatUint(uint64(track.PayloadType()), 10)
		md.MediaName.Formats = append(md.MediaName.Formats, typ)

		rtpmap, fmtp := track.Marshal()

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

func (m Media) clone() *Media {
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

func (m Media) url(contentBase *url.URL) (*url.URL, error) {
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

// Medias is a list of media streams.
type Medias []*Media

func (ms *Medias) unmarshal(mds []*psdp.MediaDescription) error {
	*ms = make(Medias, len(mds))

	for i, md := range mds {
		var m Media
		err := m.unmarshal(md)
		if err != nil {
			return fmt.Errorf("media %d is invalid: %v", i+1, err)
		}
		(*ms)[i] = &m
	}

	return nil
}

func (ms Medias) marshal(multicast bool) []byte {
	var address string
	if multicast {
		address = "224.1.0.0"
	} else {
		address = "0.0.0.0"
	}

	sout := &sdp.SessionDescription{
		SessionName: psdp.SessionName("Stream"),
		Origin: psdp.Origin{
			Username:       "-",
			NetworkType:    "IN",
			AddressType:    "IP4",
			UnicastAddress: "127.0.0.1",
		},
		// required by Darwin Streaming Server
		ConnectionInformation: &psdp.ConnectionInformation{
			NetworkType: "IN",
			AddressType: "IP4",
			Address:     &psdp.Address{Address: address},
		},
		TimeDescriptions: []psdp.TimeDescription{
			{Timing: psdp.Timing{StartTime: 0, StopTime: 0}},
		},
		MediaDescriptions: make([]*psdp.MediaDescription, len(ms)),
	}

	for i, media := range ms {
		sout.MediaDescriptions[i] = media.marshal()
	}

	byts, _ := sout.Marshal()
	return byts
}

func (ms Medias) clone() Medias {
	ret := make(Medias, len(ms))
	for i, media := range ms {
		ret[i] = media.clone()
	}
	return ret
}

func (ms Medias) setControls() {
	for i, media := range ms {
		media.Control = "mediaID=" + strconv.FormatInt(int64(i), 10)
	}
}
