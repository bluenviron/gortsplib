// Package description contains objects to describe streams.
package description

import (
	"fmt"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"

	psdp "github.com/pion/sdp/v3"

	"github.com/bluenviron/gortsplib/v4/pkg/base"
	"github.com/bluenviron/gortsplib/v4/pkg/format"
)

var smartRegexp = regexp.MustCompile("^([0-9]+) (.*?)/90000")

func replaceSmartPayloadType(payloadType string, attributes []psdp.Attribute) string {
	if payloadType == "smart/1/90000" {
		for _, attr := range attributes {
			if attr.Key == "rtpmap" {
				sm := smartRegexp.FindStringSubmatch(attr.Value)
				if sm != nil {
					return sm[1]
				}
			}
		}
	}
	return payloadType
}

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

func getFormatAttribute(attributes []psdp.Attribute, payloadType uint8, key string) string {
	for _, attr := range attributes {
		if attr.Key == key {
			v := strings.TrimSpace(attr.Value)
			if parts := strings.SplitN(v, " ", 2); len(parts) == 2 {
				if tmp, err := strconv.ParseUint(parts[0], 10, 8); err == nil && uint8(tmp) == payloadType {
					return parts[1]
				}
			}
		}
	}
	return ""
}

func decodeFMTP(enc string) map[string]string {
	if enc == "" {
		return nil
	}

	ret := make(map[string]string)

	for _, kv := range strings.Split(enc, ";") {
		kv = strings.Trim(kv, " ")

		if len(kv) == 0 {
			continue
		}

		tmp := strings.SplitN(kv, "=", 2)
		if len(tmp) != 2 {
			continue
		}

		ret[strings.ToLower(tmp[0])] = tmp[1]
	}

	return ret
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
	m.Control = getAttribute(md.Attributes, "control")

	m.Formats = nil
	for _, payloadType := range md.MediaName.Formats {
		payloadType = replaceSmartPayloadType(payloadType, md.Attributes)

		tmp, err := strconv.ParseUint(payloadType, 10, 8)
		if err != nil {
			return err
		}
		payloadTypeInt := uint8(tmp)

		rtpMap := getFormatAttribute(md.Attributes, payloadTypeInt, "rtpmap")
		fmtp := decodeFMTP(getFormatAttribute(md.Attributes, payloadTypeInt, "fmtp"))

		format, err := format.Unmarshal(string(m.Type), payloadTypeInt, rtpMap, fmtp)
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
func (m Media) Marshal() *psdp.MediaDescription {
	md := &psdp.MediaDescription{
		MediaName: psdp.MediaName{
			Media:  string(m.Type),
			Protos: []string{"RTP", "AVP"},
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

	return md
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
	if m.Control[0] != '?' && !strings.HasSuffix(strURL, "/") {
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
