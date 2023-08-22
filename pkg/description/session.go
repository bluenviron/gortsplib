package description

import (
	"fmt"

	psdp "github.com/pion/sdp/v3"

	"github.com/bluenviron/gortsplib/v4/pkg/sdp"
	"github.com/bluenviron/gortsplib/v4/pkg/url"
)

// Session is the description of a RTSP stream.
type Session struct {
	// base URL of the stream (read only).
	BaseURL *url.URL

	// title of the stream (optional).
	Title string

	// available media streams.
	Medias []*Media
}

// FindFormat finds a certain format among all the formats in all the medias of the stream.
// If the format is found, it is inserted into forma, and its media is returned.
func (d *Session) FindFormat(forma interface{}) *Media {
	for _, media := range d.Medias {
		ok := media.FindFormat(forma)
		if ok {
			return media
		}
	}
	return nil
}

// Unmarshal decodes the description from SDP.
func (d *Session) Unmarshal(ssd *sdp.SessionDescription) error {
	d.Title = string(ssd.SessionName)
	if d.Title == " " {
		d.Title = ""
	}

	d.Medias = make([]*Media, len(ssd.MediaDescriptions))

	for i, md := range ssd.MediaDescriptions {
		var m Media
		err := m.Unmarshal(md)
		if err != nil {
			return fmt.Errorf("media %d is invalid: %v", i+1, err)
		}
		d.Medias[i] = &m
	}

	return nil
}

// Marshal encodes the description in SDP.
func (d Session) Marshal(multicast bool) ([]byte, error) {
	var sessionName psdp.SessionName
	if d.Title != "" {
		sessionName = psdp.SessionName(d.Title)
	} else {
		// RFC 4566: If a session has no meaningful name, the
		// value "s= " SHOULD be used (i.e., a single space as the session name).
		sessionName = psdp.SessionName(" ")
	}

	var address string
	if multicast {
		address = "224.1.0.0"
	} else {
		address = "0.0.0.0"
	}

	sout := &sdp.SessionDescription{
		SessionName: sessionName,
		Origin: psdp.Origin{
			Username:       "-",
			NetworkType:    "IN",
			AddressType:    "IP4",
			UnicastAddress: "127.0.0.1",
		},
		// required by Darwin Sessioning Server
		ConnectionInformation: &psdp.ConnectionInformation{
			NetworkType: "IN",
			AddressType: "IP4",
			Address:     &psdp.Address{Address: address},
		},
		TimeDescriptions: []psdp.TimeDescription{
			{Timing: psdp.Timing{StartTime: 0, StopTime: 0}},
		},
		MediaDescriptions: make([]*psdp.MediaDescription, len(d.Medias)),
	}

	for i, media := range d.Medias {
		sout.MediaDescriptions[i] = media.Marshal()
	}

	return sout.Marshal()
}
