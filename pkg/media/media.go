// Package media contains the media stream definition.
package media

import (
	"fmt"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"

	psdp "github.com/pion/sdp/v3"

	"github.com/bluenviron/gortsplib/v3/pkg/formats"
	"github.com/bluenviron/gortsplib/v3/pkg/url"
)

var smartRegexp = regexp.MustCompile("^([0-9]+) (.*?)/90000")

func getControlAttribute(attributes []psdp.Attribute) string {
	for _, attr := range attributes {
		if attr.Key == "control" {
			return attr.Value
		}
	}
	return ""
}

func getDirection(attributes []psdp.Attribute) Direction {
	for _, attr := range attributes {
		switch attr.Key {
		case "sendonly":
			return DirectionSendonly

		case "recvonly":
			return DirectionRecvonly

		case "sendrecv":
			return DirectionSendrecv
		}
	}
	return ""
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

// Direction is the direction of a media stream.
type Direction string

// standard directions.
const (
	DirectionSendonly Direction = "sendonly"
	DirectionRecvonly Direction = "recvonly"
	DirectionSendrecv Direction = "sendrecv"
)

// Type is the type of a media stream.
type Type string

// standard media stream types.
const (
	TypeVideo       Type = "video"
	TypeAudio       Type = "audio"
	TypeApplication Type = "application"
)

// Media is a media stream.
// It contains one or more formats.
type Media struct {
	// Media type.
	Type Type

	// Direction of the stream.
	Direction Direction

	// Control attribute.
	Control string

	// Formats contained into the media.
	Formats []formats.Format
}

func (m *Media) unmarshal(md *psdp.MediaDescription) error {
	m.Type = Type(md.MediaName.Media)
	m.Direction = getDirection(md.Attributes)
	m.Control = getControlAttribute(md.Attributes)

	m.Formats = nil
	for _, payloadType := range md.MediaName.Formats {
		if payloadType == "smart/1/90000" {
			for _, attr := range md.Attributes {
				if attr.Key == "rtpmap" {
					sm := smartRegexp.FindStringSubmatch(attr.Value)
					if sm != nil {
						payloadType = sm[1]
						break
					}
				}
			}
		}

		tmp, err := strconv.ParseUint(payloadType, 10, 8)
		if err != nil {
			return err
		}
		payloadTypeInt := uint8(tmp)

		rtpMap := getFormatAttribute(md.Attributes, payloadTypeInt, "rtpmap")
		fmtp := decodeFMTP(getFormatAttribute(md.Attributes, payloadTypeInt, "fmtp"))

		format, err := formats.Unmarshal(string(m.Type), payloadTypeInt, rtpMap, fmtp)
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
		Attributes: []psdp.Attribute{
			{
				Key:   "control",
				Value: m.Control,
			},
		},
	}

	if m.Direction != "" {
		md.Attributes = append(md.Attributes, psdp.Attribute{
			Key: string(m.Direction),
		})
	}

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
func (m Media) URL(contentBase *url.URL) (*url.URL, error) {
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
