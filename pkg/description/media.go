// Package description contains objects to describe streams.
package description

import (
	"encoding/base64"
	"fmt"
	"reflect"
	"slices"
	"sort"
	"strconv"
	"strings"
	"unicode"

	psdp "github.com/pion/sdp/v3"

	"github.com/bluenviron/gortsplib/v5/pkg/base"
	"github.com/bluenviron/gortsplib/v5/pkg/format"
	"github.com/bluenviron/gortsplib/v5/pkg/headers"
	"github.com/bluenviron/gortsplib/v5/pkg/mikey"
)

func getAttribute(attributes []psdp.Attribute, key string) string {
	for _, attr := range attributes {
		if attr.Key == key {
			return attr.Value
		}
	}
	return ""
}

func isBackChannel(attributes []psdp.Attribute) bool {
	for _, attr := range attributes {
		if attr.Key == "sendonly" {
			return true
		}
	}
	return false
}

func sortedKeys(fmtp map[string]string) []string {
	keys := make([]string, len(fmtp))
	i := 0
	for key := range fmtp {
		keys[i] = key
		i++
	}
	sort.Strings(keys)
	return keys
}

func isAlphaNumeric(v string) bool {
	for _, r := range v {
		if !unicode.IsLetter(r) && !unicode.IsNumber(r) {
			return false
		}
	}
	return true
}

// MediaType is the type of a media stream.
type MediaType string

// media types.
const (
	MediaTypeVideo       MediaType = "video"
	MediaTypeAudio       MediaType = "audio"
	MediaTypeApplication MediaType = "application"
)

// Media is a media stream.
// It contains one or more formats.
type Media struct {
	// Media type.
	Type MediaType

	// Media ID (optional).
	ID string

	// Whether this media is a back channel.
	IsBackChannel bool

	// RTP Profile.
	Profile headers.TransportProfile

	// key-mgmt attribute.
	KeyMgmtMikey *mikey.Message

	// Control attribute.
	Control string

	// Formats contained into the media.
	Formats []format.Format
}

// Unmarshal decodes the media from the SDP format.
func (m *Media) Unmarshal(md *psdp.MediaDescription) error {
	m.Type = MediaType(md.MediaName.Media)

	m.ID = getAttribute(md.Attributes, "mid")
	if m.ID != "" && !isAlphaNumeric(m.ID) {
		return fmt.Errorf("invalid mid: %v", m.ID)
	}

	m.IsBackChannel = isBackChannel(md.Attributes)

	if slices.Contains(md.MediaName.Protos, "SAVP") {
		m.Profile = headers.TransportProfileSAVP
	} else {
		m.Profile = headers.TransportProfileAVP
	}

	if enc := getAttribute(md.Attributes, "key-mgmt"); enc != "" {
		if !strings.HasPrefix(enc, "mikey ") {
			return fmt.Errorf("unsupported key-mgmt: %v", enc)
		}

		enc2, err := base64.StdEncoding.DecodeString(enc[len("mikey "):])
		if err != nil {
			return err
		}

		m.KeyMgmtMikey = &mikey.Message{}
		err = m.KeyMgmtMikey.Unmarshal(enc2)
		if err != nil {
			return err
		}
	}

	m.Control = getAttribute(md.Attributes, "control")

	m.Formats = nil

	for _, payloadType := range md.MediaName.Formats {
		format, err := format.Unmarshal(md, payloadType)
		if err != nil {
			return err
		}

		m.Formats = append(m.Formats, format)
	}

	if m.Formats == nil {
		return fmt.Errorf("no formats found")
	}

	return nil
}

// Marshal encodes the media in SDP format.
func (m Media) Marshal() (*psdp.MediaDescription, error) {
	var protos []string

	if m.Profile == headers.TransportProfileSAVP {
		protos = []string{"RTP", "SAVP"}
	} else {
		protos = []string{"RTP", "AVP"}
	}

	md := &psdp.MediaDescription{
		MediaName: psdp.MediaName{
			Media:  string(m.Type),
			Protos: protos,
		},
	}

	if m.ID != "" {
		md.Attributes = append(md.Attributes, psdp.Attribute{
			Key:   "mid",
			Value: m.ID,
		})
	}

	if m.IsBackChannel {
		md.Attributes = append(md.Attributes, psdp.Attribute{
			Key: "sendonly",
		})
	}

	if m.KeyMgmtMikey != nil {
		keyEnc, err := m.KeyMgmtMikey.Marshal()
		if err != nil {
			return nil, err
		}

		md.Attributes = append(md.Attributes, psdp.Attribute{
			Key:   "key-mgmt",
			Value: "mikey " + base64.StdEncoding.EncodeToString(keyEnc),
		})
	}

	md.Attributes = append(md.Attributes, psdp.Attribute{
		Key:   "control",
		Value: m.Control,
	})

	for _, forma := range m.Formats {
		typ := strconv.FormatUint(uint64(forma.PayloadType()), 10)
		md.MediaName.Formats = append(md.MediaName.Formats, typ)

		rtpmap := forma.RTPMap()
		if rtpmap != "" {
			md.Attributes = append(md.Attributes, psdp.Attribute{
				Key:   "rtpmap",
				Value: typ + " " + rtpmap,
			})
		}

		fmtp := forma.FMTP()
		if len(fmtp) != 0 {
			tmp := make([]string, len(fmtp))
			for i, key := range sortedKeys(fmtp) {
				tmp[i] = key + "=" + fmtp[key]
			}

			md.Attributes = append(md.Attributes, psdp.Attribute{
				Key:   "fmtp",
				Value: typ + " " + strings.Join(tmp, "; "),
			})
		}
	}

	return md, nil
}

// URL returns the absolute URL of the media.
func (m Media) URL(contentBase *base.URL) (*base.URL, error) {
	if contentBase == nil {
		return nil, fmt.Errorf("Content-Base header not provided")
	}

	// no control attribute, use base URL
	if m.Control == "" {
		return contentBase, nil
	}

	// control attribute contains an absolute path
	if strings.HasPrefix(m.Control, "rtsp://") ||
		strings.HasPrefix(m.Control, "rtsps://") {
		ur, err := base.ParseURL(m.Control)
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
	if m.Control[0] != '?' && m.Control[0] != '/' && !strings.HasSuffix(strURL, "/") {
		strURL += "/"
	}

	ur, _ := base.ParseURL(strURL + m.Control)
	return ur, nil
}

// FindFormat finds a certain format among all the formats in the media.
func (m Media) FindFormat(forma interface{}) bool {
	for _, formak := range m.Formats {
		if reflect.TypeOf(formak) == reflect.TypeOf(forma).Elem() {
			reflect.ValueOf(forma).Elem().Set(reflect.ValueOf(formak))
			return true
		}
	}
	return false
}
