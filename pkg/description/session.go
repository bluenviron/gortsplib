package description

import (
	"encoding/base64"
	"fmt"
	"strings"

	psdp "github.com/pion/sdp/v3"

	"github.com/bluenviron/gortsplib/v4/pkg/base"
	"github.com/bluenviron/gortsplib/v4/pkg/mikey"
	"github.com/bluenviron/gortsplib/v4/pkg/sdp"
)

func atLeastOneHasMID(medias []*Media) bool {
	for _, media := range medias {
		if media.ID != "" {
			return true
		}
	}
	return false
}

func atLeastOneDoesntHaveMID(medias []*Media) bool {
	for _, media := range medias {
		if media.ID == "" {
			return true
		}
	}
	return false
}

func hasMediaWithID(medias []*Media, id string) bool {
	for _, media := range medias {
		if media.ID == id {
			return true
		}
	}
	return false
}

// SessionFECGroup is a FEC group.
type SessionFECGroup []string

// Session is the description of a RTSP stream.
type Session struct {
	// Base URL of the stream (read only).
	BaseURL *base.URL

	// Title of the stream (optional).
	Title string

	// Whether to use multicast.
	Multicast bool

	// key-mgmt attribute.
	KeyMgmtMikey *mikey.Message

	// FEC groups (RFC5109).
	FECGroups []SessionFECGroup

	// Media streams.
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

	if enc := getAttribute(ssd.Attributes, "key-mgmt"); enc != "" {
		if !strings.HasPrefix(enc, "mikey ") {
			return fmt.Errorf("unsupported key-mgmt: %v", enc)
		}

		enc2, err := base64.StdEncoding.DecodeString(enc[len("mikey "):])
		if err != nil {
			return err
		}

		d.KeyMgmtMikey = &mikey.Message{}
		err = d.KeyMgmtMikey.Unmarshal(enc2)
		if err != nil {
			return err
		}
	}

	if len(ssd.MediaDescriptions) == 0 {
		return fmt.Errorf("no media streams are present in SDP")
	}

	d.Medias = make([]*Media, len(ssd.MediaDescriptions))

	for i, md := range ssd.MediaDescriptions {
		var m Media
		err := m.Unmarshal(md)
		if err != nil {
			return fmt.Errorf("media %d is invalid: %w", i+1, err)
		}

		if m.ID != "" && hasMediaWithID(d.Medias[:i], m.ID) {
			return fmt.Errorf("duplicate media IDs")
		}

		d.Medias[i] = &m
	}

	if atLeastOneHasMID(d.Medias) && atLeastOneDoesntHaveMID(d.Medias) {
		return fmt.Errorf("media IDs sent partially")
	}

	for _, attr := range ssd.Attributes {
		if attr.Key == "group" && strings.HasPrefix(attr.Value, "FEC ") {
			group := SessionFECGroup(strings.Split(attr.Value[len("FEC "):], " "))

			for _, id := range group {
				if !hasMediaWithID(d.Medias, id) {
					return fmt.Errorf("FEC group points to an invalid media ID: %v", id)
				}
			}

			d.FECGroups = append(d.FECGroups, group)
		}
	}

	return nil
}

// Marshal encodes the description in SDP format.
// The argument is deprecated and has no effect. Set Session.Multicast to enable multicast.
func (d Session) Marshal(_ bool) ([]byte, error) {
	var sessionName psdp.SessionName
	if d.Title != "" {
		sessionName = psdp.SessionName(d.Title)
	} else {
		// RFC 4566: If a session has no meaningful name, the
		// value "s= " SHOULD be used (i.e., a single space as the session name).
		sessionName = psdp.SessionName(" ")
	}

	var address string
	if d.Multicast {
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
		// required by Darwin Streaming Server
		ConnectionInformation: &psdp.ConnectionInformation{
			NetworkType: "IN",
			AddressType: "IP4",
			Address:     &psdp.Address{Address: address},
		},
		TimeDescriptions: []psdp.TimeDescription{
			{Timing: psdp.Timing{StartTime: 0, StopTime: 0}},
		},
	}

	for _, group := range d.FECGroups {
		sout.Attributes = append(sout.Attributes, psdp.Attribute{
			Key:   "group",
			Value: "FEC " + strings.Join(group, " "),
		})
	}

	if d.KeyMgmtMikey != nil {
		keyEnc, err := d.KeyMgmtMikey.Marshal()
		if err != nil {
			return nil, err
		}

		sout.Attributes = append(sout.Attributes, psdp.Attribute{
			Key:   "key-mgmt",
			Value: "mikey " + base64.StdEncoding.EncodeToString(keyEnc),
		})
	}

	sout.MediaDescriptions = make([]*psdp.MediaDescription, len(d.Medias))

	for i, media := range d.Medias {
		med, err := media.Marshal2()
		if err != nil {
			return nil, err
		}
		sout.MediaDescriptions[i] = med
	}

	out, _ := sout.Marshal()

	return out, nil
}
