// Package sdp contains a SDP encoder/decoder compatible with most RTSP implementations.
package sdp

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	psdp "github.com/pion/sdp/v3"
)

// SessionDescription is a SDP session description.
type SessionDescription psdp.SessionDescription

// Attribute returns the value of an attribute and if it exists
func (s *SessionDescription) Attribute(key string) (string, bool) {
	return (*psdp.SessionDescription)(s).Attribute(key)
}

// Marshal encodes a SessionDescription.
func (s *SessionDescription) Marshal() ([]byte, error) {
	return (*psdp.SessionDescription)(s).Marshal()
}

var (
	errSDPInvalidSyntax       = errors.New("sdp: invalid syntax")
	errSDPInvalidNumericValue = errors.New("sdp: invalid numeric value")
	errSDPInvalidValue        = errors.New("sdp: invalid value")
	errSDPInvalidPortValue    = errors.New("sdp: invalid port value")
)

func indexOf(element string, data []string) int {
	for k, v := range data {
		if element == v {
			return k
		}
	}
	return -1
}

func parsePort(value string) (int, error) {
	port, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("%w `%v`", errSDPInvalidPortValue, port)
	}

	if port < 0 || port > 65536 {
		return 0, fmt.Errorf("%w -- out of range `%v`", errSDPInvalidPortValue, port)
	}

	return port, nil
}

func (s *SessionDescription) unmarshalVersion(value string) error {
	if value != "0" {
		return fmt.Errorf("invalid version")
	}
	return nil
}

func (s *SessionDescription) unmarshalSessionName(value string) error {
	s.SessionName = psdp.SessionName(value)
	return nil
}

func (s *SessionDescription) unmarshalOrigin(value string) error {
	// special case for live reporter app
	if value[:3] == "-0 " {
		value = "- 0 " + value[3:]
	}

	// find spaces from end to beginning, to support multiple spaces
	// in the first field
	fields := func() []string {
		values := strings.Split(strings.TrimSpace(value), " ")

		// special case for some onvif2 cameras
		if strings.Compare(values[len(values)-1], "IP4") == 0 {
			values = append(values, "127.0.0.1")
		}

		return append([]string{strings.Join(values[0:len(values)-5], " ")}, values[len(values)-5:]...)
	}()

	if len(fields) != 6 {
		return fmt.Errorf("%w `o=%v`", errSDPInvalidSyntax, fields)
	}

	var sessionID uint64
	var err error
	switch {
	case strings.HasPrefix(fields[1], "0x") || strings.HasPrefix(fields[1], "0X"):
		sessionID, err = strconv.ParseUint(fields[1][2:], 16, 64)
	case strings.ContainsAny(fields[1], "abcdefABCDEF"):
		sessionID, err = strconv.ParseUint(fields[1], 16, 64)
	default:
		sessionID, err = strconv.ParseUint(fields[1], 10, 64)
	}
	if err != nil {
		return fmt.Errorf("%w `%v`", errSDPInvalidNumericValue, fields[1])
	}

	sessionVersion, err := strconv.ParseUint(fields[2], 10, 64)
	if err != nil {
		return fmt.Errorf("%w `%v`", errSDPInvalidNumericValue, fields[2])
	}

	// Set according to currently registered with IANA
	// https://tools.ietf.org/html/rfc4566#section-8.2.6
	if i := indexOf(fields[3], []string{"IN"}); i == -1 {
		return fmt.Errorf("%w `%v`", errSDPInvalidValue, fields[3])
	}

	// Set according to currently registered with IANA
	// https://tools.ietf.org/html/rfc4566#section-8.2.7
	if i := indexOf(fields[4], []string{"IP4", "IP6"}); i == -1 {
		return fmt.Errorf("%w `%v`", errSDPInvalidValue, fields[3])
	}

	s.Origin = psdp.Origin{
		Username:       fields[0],
		SessionID:      sessionID,
		SessionVersion: sessionVersion,
		NetworkType:    fields[3],
		AddressType:    fields[4],
		UnicastAddress: fields[5],
	}

	return nil
}

func (s *SessionDescription) unmarshalSessionInformation(value string) error {
	sessionInformation := psdp.Information(value)
	s.SessionInformation = &sessionInformation
	return nil
}

func (s *SessionDescription) unmarshalURI(value string) error {
	var err error
	s.URI, err = url.Parse(value)
	if err != nil {
		return err
	}

	return nil
}

func (s *SessionDescription) unmarshalEmail(value string) error {
	emailAddress := psdp.EmailAddress(value)
	s.EmailAddress = &emailAddress
	return nil
}

func (s *SessionDescription) unmarshalPhone(value string) error {
	phoneNumber := psdp.PhoneNumber(value)
	s.PhoneNumber = &phoneNumber
	return nil
}

func unmarshalConnectionInformation(value string) (*psdp.ConnectionInformation, error) {
	// special case for some hikvision cameras
	if strings.HasPrefix(value, "IN c=IN") {
		value = value[len("IN c="):]
	}

	fields := strings.Fields(value)
	if len(fields) < 2 {
		return nil, fmt.Errorf("%w `c=%v`", errSDPInvalidSyntax, fields)
	}

	// Set according to currently registered with IANA
	// https://tools.ietf.org/html/rfc4566#section-8.2.6
	if i := indexOf(fields[0], []string{"IN"}); i == -1 {
		return nil, fmt.Errorf("%w `%v`", errSDPInvalidValue, fields[0])
	}

	// Set according to currently registered with IANA
	// https://tools.ietf.org/html/rfc4566#section-8.2.7
	if i := indexOf(fields[1], []string{"IP4", "IP6"}); i == -1 {
		return nil, fmt.Errorf("%w `%v`", errSDPInvalidValue, fields[1])
	}

	connAddr := new(psdp.Address)
	if len(fields) > 2 {
		connAddr.Address = fields[2]
	}

	return &psdp.ConnectionInformation{
		NetworkType: fields[0],
		AddressType: fields[1],
		Address:     connAddr,
	}, nil
}

func (s *SessionDescription) unmarshalSessionConnectionInformation(value string) error {
	var err error
	s.ConnectionInformation, err = unmarshalConnectionInformation(value)
	if err != nil {
		return fmt.Errorf("%w `c=%v`", errSDPInvalidSyntax, value)
	}
	return nil
}

func unmarshalBandwidth(value string) (*psdp.Bandwidth, error) {
	parts := strings.Split(value, ":")
	if len(parts) != 2 {
		return nil, fmt.Errorf("%w `b=%v`", errSDPInvalidValue, parts)
	}

	experimental := strings.HasPrefix(parts[0], "X-")
	if experimental {
		parts[0] = strings.TrimPrefix(parts[0], "X-")
	} else if i := indexOf(parts[0], []string{"CT", "AS", "RR", "RS"}); i == -1 {
		// Set according to currently registered with IANA
		// https://tools.ietf.org/html/rfc4566#section-5.8
		return nil, fmt.Errorf("%w `%v`", errSDPInvalidValue, parts[0])
	}

	bandwidth, err := strconv.ParseUint(parts[1], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("%w `%v`", errSDPInvalidNumericValue, parts[1])
	}

	return &psdp.Bandwidth{
		Experimental: experimental,
		Type:         parts[0],
		Bandwidth:    bandwidth,
	}, nil
}

func (s *SessionDescription) unmarshalSessionBandwidth(value string) error {
	bandwidth, err := unmarshalBandwidth(value)
	if err != nil {
		return fmt.Errorf("%w `b=%v`", errSDPInvalidValue, value)
	}
	s.Bandwidth = append(s.Bandwidth, *bandwidth)

	return nil
}

func (s *SessionDescription) unmarshalTimeZones(value string) error {
	// These fields are transimitted in pairs
	// z=<adjustment time> <offset> <adjustment time> <offset> ....
	// so we are making sure that there are actually multiple of 2 total.
	fields := strings.Fields(value)
	if len(fields)%2 != 0 {
		return fmt.Errorf("%w `t=%v`", errSDPInvalidSyntax, fields)
	}

	for i := 0; i < len(fields); i += 2 {
		var timeZone psdp.TimeZone

		var err error
		timeZone.AdjustmentTime, err = strconv.ParseUint(fields[i], 10, 64)
		if err != nil {
			return fmt.Errorf("%w `%v`", errSDPInvalidValue, fields)
		}

		timeZone.Offset, err = parseTimeUnits(fields[i+1])
		if err != nil {
			return err
		}

		s.TimeZones = append(s.TimeZones, timeZone)
	}

	return nil
}

func (s *SessionDescription) unmarshalSessionEncryptionKey(value string) error {
	encryptionKey := psdp.EncryptionKey(value)
	s.EncryptionKey = &encryptionKey
	return nil
}

func (s *SessionDescription) unmarshalSessionAttribute(value string) error {
	i := strings.IndexRune(value, ':')
	var a psdp.Attribute
	if i > 0 {
		a = psdp.NewAttribute(value[:i], value[i+1:])
	} else {
		a = psdp.NewPropertyAttribute(value)
	}

	s.Attributes = append(s.Attributes, a)
	return nil
}

func (s *SessionDescription) unmarshalTiming(value string) error {
	if value == "now-" {
		// special case for some FLIR cameras with invalid timing element
		value = "0 0"
	}
	fields := strings.Fields(value)
	if len(fields) < 2 {
		return fmt.Errorf("%w `t=%v`", errSDPInvalidSyntax, fields)
	}

	td := psdp.TimeDescription{}

	var err error
	td.Timing.StartTime, err = strconv.ParseUint(fields[0], 10, 64)
	if err != nil {
		return fmt.Errorf("%w `%v`", errSDPInvalidNumericValue, fields[1])
	}

	td.Timing.StopTime, err = strconv.ParseUint(fields[1], 10, 64)
	if err != nil {
		return fmt.Errorf("%w `%v`", errSDPInvalidNumericValue, fields[1])
	}

	s.TimeDescriptions = append(s.TimeDescriptions, td)

	return nil
}

func parseTimeUnits(value string) (int64, error) {
	// Some time offsets in the protocol can be provided with a shorthand
	// notation. This code ensures to convert it to NTP timestamp format.
	//      d - days (86400 seconds)
	//      h - hours (3600 seconds)
	//      m - minutes (60 seconds)
	//      s - seconds (allowed for completeness)
	switch value[len(value)-1:] {
	case "d":
		num, err := strconv.ParseInt(value[:len(value)-1], 10, 64)
		if err != nil {
			return 0, fmt.Errorf("%w `%v`", errSDPInvalidValue, value)
		}
		return num * 86400, nil
	case "h":
		num, err := strconv.ParseInt(value[:len(value)-1], 10, 64)
		if err != nil {
			return 0, fmt.Errorf("%w `%v`", errSDPInvalidValue, value)
		}
		return num * 3600, nil
	case "m":
		num, err := strconv.ParseInt(value[:len(value)-1], 10, 64)
		if err != nil {
			return 0, fmt.Errorf("%w `%v`", errSDPInvalidValue, value)
		}
		return num * 60, nil
	}

	num, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%w `%v`", errSDPInvalidValue, value)
	}

	return num, nil
}

func (s *SessionDescription) unmarshalRepeatTimes(value string) error {
	fields := strings.Fields(value)
	if len(fields) < 3 {
		return fmt.Errorf("%w `r=%v`", errSDPInvalidSyntax, fields)
	}

	latestTimeDesc := &s.TimeDescriptions[len(s.TimeDescriptions)-1]

	newRepeatTime := psdp.RepeatTime{}
	var err error
	newRepeatTime.Interval, err = parseTimeUnits(fields[0])
	if err != nil {
		return fmt.Errorf("%w `%v`", errSDPInvalidValue, fields)
	}

	newRepeatTime.Duration, err = parseTimeUnits(fields[1])
	if err != nil {
		return fmt.Errorf("%w `%v`", errSDPInvalidValue, fields)
	}

	for i := 2; i < len(fields); i++ {
		offset, err := parseTimeUnits(fields[i])
		if err != nil {
			return fmt.Errorf("%w `%v`", errSDPInvalidValue, fields)
		}
		newRepeatTime.Offsets = append(newRepeatTime.Offsets, offset)
	}
	latestTimeDesc.RepeatTimes = append(latestTimeDesc.RepeatTimes, newRepeatTime)

	return nil
}

func (s *SessionDescription) unmarshalMediaDescription(value string) error {
	fields := strings.Fields(value)
	if len(fields) < 4 {
		return fmt.Errorf("%w `m=%v`", errSDPInvalidSyntax, fields)
	}

	newMediaDesc := &psdp.MediaDescription{}

	// <media>
	// Set according to currently registered with IANA
	// https://tools.ietf.org/html/rfc4566#section-5.14
	if i := indexOf(fields[0], []string{"audio", "video", "text", "application", "message"}); i == -1 {
		return fmt.Errorf("%w `%v`", errSDPInvalidValue, fields[0])
	}
	newMediaDesc.MediaName.Media = fields[0]

	// <port>
	parts := strings.Split(fields[1], "/")
	var err error
	newMediaDesc.MediaName.Port.Value, err = parsePort(parts[0])
	if err != nil {
		return fmt.Errorf("%w `%v`", errSDPInvalidPortValue, parts[0])
	}

	if len(parts) > 1 {
		portRange, err := strconv.Atoi(parts[1])
		if err != nil {
			return fmt.Errorf("%w `%v`", errSDPInvalidValue, parts)
		}
		newMediaDesc.MediaName.Port.Range = &portRange
	}

	// <proto>
	// Set according to currently registered with IANA
	// https://tools.ietf.org/html/rfc4566#section-5.14
	for _, proto := range strings.Split(fields[2], "/") {
		if i := indexOf(proto, []string{
			"UDP", "RTP", "AVP", "SAVP", "SAVPF",
			"MP2T", "TLS", "DTLS", "SCTP", "AVPF", "TCP",
		}); i == -1 {
			return fmt.Errorf("%w `%v`", errSDPInvalidNumericValue, fields[2])
		}
		newMediaDesc.MediaName.Protos = append(newMediaDesc.MediaName.Protos, proto)
	}

	// <fmt>...
	for i := 3; i < len(fields); i++ {
		newMediaDesc.MediaName.Formats = append(newMediaDesc.MediaName.Formats, fields[i])
	}

	s.MediaDescriptions = append(s.MediaDescriptions, newMediaDesc)

	return nil
}

func (s *SessionDescription) unmarshalMediaTitle(value string) error {
	latestMediaDesc := s.MediaDescriptions[len(s.MediaDescriptions)-1]
	mediaTitle := psdp.Information(value)
	latestMediaDesc.MediaTitle = &mediaTitle
	return nil
}

func (s *SessionDescription) unmarshalMediaConnectionInformation(value string) error {
	latestMediaDesc := s.MediaDescriptions[len(s.MediaDescriptions)-1]
	var err error
	latestMediaDesc.ConnectionInformation, err = unmarshalConnectionInformation(value)
	if err != nil {
		return fmt.Errorf("%w `c=%v`", errSDPInvalidSyntax, value)
	}

	return nil
}

func (s *SessionDescription) unmarshalMediaBandwidth(value string) error {
	latestMediaDesc := s.MediaDescriptions[len(s.MediaDescriptions)-1]
	bandwidth, err := unmarshalBandwidth(value)
	if err != nil {
		return fmt.Errorf("%w `b=%v`", errSDPInvalidSyntax, value)
	}
	latestMediaDesc.Bandwidth = append(latestMediaDesc.Bandwidth, *bandwidth)
	return nil
}

func (s *SessionDescription) unmarshalMediaEncryptionKey(value string) error {
	latestMediaDesc := s.MediaDescriptions[len(s.MediaDescriptions)-1]
	encryptionKey := psdp.EncryptionKey(value)
	latestMediaDesc.EncryptionKey = &encryptionKey
	return nil
}

func (s *SessionDescription) unmarshalMediaAttribute(value string) error {
	i := strings.IndexRune(value, ':')
	var a psdp.Attribute
	if i > 0 {
		a = psdp.NewAttribute(value[:i], value[i+1:])
	} else {
		a = psdp.NewPropertyAttribute(value)
	}

	latestMediaDesc := s.MediaDescriptions[len(s.MediaDescriptions)-1]
	latestMediaDesc.Attributes = append(latestMediaDesc.Attributes, a)
	return nil
}

// Unmarshal decodes a SessionDescription.
// This is rewritten from scratch to guarantee compatibility with most RTSP
// implementations.
func (s *SessionDescription) Unmarshal(byts []byte) error {
	str := string(byts)

	type stateVal int

	const (
		stateInitial stateVal = iota
		stateSession
		stateMedia
	)

	state := stateInitial

	for _, line := range strings.Split(strings.ReplaceAll(str, "\r", ""), "\n") {
		if line == "" {
			continue
		}

		if len(line) < 2 || line[1] != '=' {
			return fmt.Errorf("invalid line: (%s)", line)
		}

		key := line[0]
		val := line[2:]

		switch state {
		case stateInitial:
			switch key {
			case 'v':
				err := s.unmarshalVersion(val)
				if err != nil {
					return fmt.Errorf("%s (%s)", err, line)
				}
				state = stateSession

			default:
				return fmt.Errorf("invalid key: %c (%s)", key, line)
			}

		case stateSession:
			switch key {
			case 'o':
				err := s.unmarshalOrigin(val)
				if err != nil {
					return fmt.Errorf("%s (%s)", err, line)
				}

			case 's':
				err := s.unmarshalSessionName(val)
				if err != nil {
					return fmt.Errorf("%s (%s)", err, line)
				}

			case 'i':
				err := s.unmarshalSessionInformation(val)
				if err != nil {
					return fmt.Errorf("%s (%s)", err, line)
				}

			case 'u':
				err := s.unmarshalURI(val)
				if err != nil {
					return fmt.Errorf("%s (%s)", err, line)
				}

			case 'e':
				err := s.unmarshalEmail(val)
				if err != nil {
					return fmt.Errorf("%s (%s)", err, line)
				}

			case 'p':
				err := s.unmarshalPhone(val)
				if err != nil {
					return fmt.Errorf("%s (%s)", err, line)
				}

			case 'c':
				err := s.unmarshalSessionConnectionInformation(val)
				if err != nil {
					return fmt.Errorf("%s (%s)", err, line)
				}

			case 'b':
				err := s.unmarshalSessionBandwidth(val)
				if err != nil {
					return fmt.Errorf("%s (%s)", err, line)
				}

			case 'z':
				err := s.unmarshalTimeZones(val)
				if err != nil {
					return fmt.Errorf("%s (%s)", err, line)
				}

			case 'k':
				err := s.unmarshalSessionEncryptionKey(val)
				if err != nil {
					return fmt.Errorf("%s (%s)", err, line)
				}

			case 'a':
				err := s.unmarshalSessionAttribute(val)
				if err != nil {
					return fmt.Errorf("%s (%s)", err, line)
				}

			case 't':
				err := s.unmarshalTiming(val)
				if err != nil {
					return fmt.Errorf("%s (%s)", err, line)
				}

			case 'r':
				err := s.unmarshalRepeatTimes(val)
				if err != nil {
					return fmt.Errorf("%s (%s)", err, line)
				}

			case 'm':
				err := s.unmarshalMediaDescription(val)
				if err != nil {
					return fmt.Errorf("%s (%s)", err, line)
				}
				state = stateMedia

			default:
				return fmt.Errorf("invalid key: %c (%s)", key, line)
			}

		case stateMedia:
			switch key {
			case 'm':
				err := s.unmarshalMediaDescription(val)
				if err != nil {
					return fmt.Errorf("%s (%s)", err, line)
				}

			case 'i':
				err := s.unmarshalMediaTitle(val)
				if err != nil {
					return fmt.Errorf("%s (%s)", err, line)
				}

			case 'c':
				err := s.unmarshalMediaConnectionInformation(val)
				if err != nil {
					return fmt.Errorf("%s (%s)", err, line)
				}

			case 'b':
				err := s.unmarshalMediaBandwidth(val)
				if err != nil {
					return fmt.Errorf("%s (%s)", err, line)
				}

			case 'k':
				err := s.unmarshalMediaEncryptionKey(val)
				if err != nil {
					return fmt.Errorf("%s (%s)", err, line)
				}

			case 'a':
				err := s.unmarshalMediaAttribute(val)
				if err != nil {
					return fmt.Errorf("%s (%s)", err, line)
				}

			default:
				return fmt.Errorf("invalid key: %c (%s)", key, line)
			}
		}
	}

	return nil
}
