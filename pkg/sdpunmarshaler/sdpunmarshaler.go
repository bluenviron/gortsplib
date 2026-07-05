// Package sdpunmarshaler contains a SDP unmarshaler compatible with most RTSP implementations.
package sdpunmarshaler

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/pion/sdp/v3"
)

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

func anyOf(element string, data ...string) bool {
	return slices.Contains(data, element)
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

func stringsReverseIndexByte(s string, b byte) int {
	for i := len(s) - 2; i >= 0; i-- {
		if s[i] == b {
			return i
		}
	}
	return -1
}

func unmarshalProtocolVersion(_ *sdp.SessionDescription, value string) error {
	if value != "0" {
		return fmt.Errorf("invalid version")
	}

	return nil
}

func unmarshalSessionName(s *sdp.SessionDescription, value string) error {
	s.SessionName = sdp.SessionName(value)
	return nil
}

func unmarshalOrigin(s *sdp.SessionDescription, value string) error {
	value = strings.Replace(value, " IN IPV4 ", " IN IP4 ", 1)
	value = strings.Replace(value, " IP IP4 ", " IN IP4 ", 1)
	value = strings.Replace(value, " IP IP6 ", " IN IP6 ", 1)

	if strings.HasSuffix(value, " IN") {
		value += " IP4"
	} else if strings.HasSuffix(value, " IN ") {
		value += "IP4"
	}

	if strings.HasSuffix(value, "IN IP4") {
		value += " "
	}

	i := strings.Index(value, " IN IP4 ")
	if i < 0 {
		i = strings.Index(value, " IN IP6 ")
		if i < 0 {
			return fmt.Errorf("%w `o=%v`", errSDPInvalidSyntax, value)
		}
	}

	s.Origin.NetworkType = value[i+1 : i+3]
	s.Origin.AddressType = value[i+4 : i+7]
	s.Origin.UnicastAddress = strings.TrimSpace(value[i+8:])
	value = value[:i]

	i = stringsReverseIndexByte(value, ' ')
	if i < 0 {
		return fmt.Errorf("%w `o=%v`", errSDPInvalidSyntax, value)
	}

	var tmp string
	tmp, value = value[i+1:], value[:i]

	if i = strings.Index(tmp, "."); i >= 0 {
		tmp = tmp[:i]
	}
	tmp = strings.TrimPrefix(tmp, "-")

	var err error
	s.Origin.SessionVersion, err = strconv.ParseUint(tmp, 10, 64)
	if err != nil {
		return fmt.Errorf("%w `%v`", errSDPInvalidNumericValue, tmp)
	}

	if value == "-0" { // live reporter app
		value = "- 0"
	}

	i = stringsReverseIndexByte(value, ' ')
	if i < 0 {
		return nil
	}

	tmp, value = value[i+1:], value[:i]

	switch {
	case strings.HasPrefix(tmp, "0x"), strings.HasPrefix(tmp, "0X"):
		s.Origin.SessionID, err = strconv.ParseUint(tmp[2:], 16, 64)
	case strings.ContainsAny(tmp, "abcdefABCDEF"):
		s.Origin.SessionID, err = strconv.ParseUint(tmp, 16, 64)
	default:
		if i = strings.Index(tmp, "."); i >= 0 {
			tmp = tmp[:i]
		}
		tmp = strings.TrimPrefix(tmp, "-")

		s.Origin.SessionID, err = strconv.ParseUint(tmp, 10, 64)
	}
	if err != nil {
		return fmt.Errorf("%w `%v`", errSDPInvalidNumericValue, tmp)
	}

	s.Origin.Username = value

	return nil
}

func unmarshalSessionInformation(s *sdp.SessionDescription, value string) error {
	sessionInformation := sdp.Information(value)
	s.SessionInformation = &sessionInformation
	return nil
}

func unmarshalURI(s *sdp.SessionDescription, value string) error {
	var err error
	s.URI, err = url.Parse(value)
	if err != nil {
		return err
	}

	return nil
}

func unmarshalEmail(s *sdp.SessionDescription, value string) error {
	emailAddress := sdp.EmailAddress(value)
	s.EmailAddress = &emailAddress
	return nil
}

func unmarshalPhone(s *sdp.SessionDescription, value string) error {
	phoneNumber := sdp.PhoneNumber(value)
	s.PhoneNumber = &phoneNumber
	return nil
}

func unmarshalConnectionInformation(value string) (*sdp.ConnectionInformation, error) {
	if strings.TrimSpace(value) == "IN" {
		return nil, nil
	}

	value = strings.Replace(value, "IN IPV4 ", "IN IP4 ", 1)

	if strings.HasPrefix(value, "IN c=IN") {
		value = value[len("IN c="):]
	}

	fields := strings.Fields(value)
	if len(fields) < 2 {
		return nil, fmt.Errorf("%w `c=%v`", errSDPInvalidSyntax, fields)
	}

	// Tolerate malformed lines like:
	// c=IN 192.168.4.232
	// c=IN fe80::1234
	if len(fields) == 2 && strings.EqualFold(fields[0], "IN") {
		if ip := net.ParseIP(fields[1]); ip != nil {
			addrType := "IP6"
			if ip.To4() != nil {
				addrType = "IP4"
			}
			fields = []string{"IN", addrType, fields[1]}
		}
	}

	// Set according to currently registered with IANA
	// https://tools.ietf.org/html/rfc4566#section-8.2.6
	if i := indexOf(strings.ToUpper(fields[0]), []string{"IN"}); i == -1 {
		return nil, fmt.Errorf("%w `%v`", errSDPInvalidValue, fields[0])
	}

	// Set according to currently registered with IANA
	// https://tools.ietf.org/html/rfc4566#section-8.2.7
	if i := indexOf(fields[1], []string{"IP4", "IP6"}); i == -1 {
		return nil, fmt.Errorf("%w `%v`", errSDPInvalidValue, fields[1])
	}

	connAddr := new(sdp.Address)
	if len(fields) > 2 {
		connAddr.Address = fields[2]
	}

	return &sdp.ConnectionInformation{
		NetworkType: strings.ToUpper(fields[0]),
		AddressType: fields[1],
		Address:     connAddr,
	}, nil
}

func unmarshalSessionConnectionInformation(s *sdp.SessionDescription, value string) error {
	var err error
	s.ConnectionInformation, err = unmarshalConnectionInformation(value)
	if err != nil {
		return fmt.Errorf("%w `c=%v`", errSDPInvalidSyntax, value)
	}

	return nil
}

func unmarshalBandwidth(_ *sdp.SessionDescription, value string) (*sdp.Bandwidth, error) {
	parts := strings.Split(value, ":")
	if len(parts) != 2 {
		return nil, fmt.Errorf("%w `b=%v`", errSDPInvalidValue, parts)
	}

	experimental := strings.HasPrefix(parts[0], "X-")
	if experimental {
		parts[0] = strings.TrimPrefix(parts[0], "X-")
	} else if !anyOf(parts[0], "CT", "AS", "TIAS", "RS", "RR") {
		// Set according to currently registered with IANA
		// https://tools.ietf.org/html/rfc4566#section-5.8
		// https://tools.ietf.org/html/rfc3890#section-6.2
		// https://tools.ietf.org/html/rfc3556#section-2
		return nil, fmt.Errorf("%w `%v`", errSDPInvalidValue, parts[0])
	}

	bandwidth, err := strconv.ParseUint(parts[1], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("%w `%v`", errSDPInvalidNumericValue, parts[1])
	}

	return &sdp.Bandwidth{
		Experimental: experimental,
		Type:         parts[0],
		Bandwidth:    bandwidth,
	}, nil
}

func unmarshalSessionBandwidth(s *sdp.SessionDescription, value string) error {
	bandwidth, err := unmarshalBandwidth(s, value)
	if err != nil {
		return fmt.Errorf("%w `b=%v`", errSDPInvalidValue, value)
	}
	s.Bandwidth = append(s.Bandwidth, *bandwidth)

	return nil
}

func unmarshalTimeZones(s *sdp.SessionDescription, value string) error {
	// These fields are transimitted in pairs
	// z=<adjustment time> <offset> <adjustment time> <offset> ....
	// so we are making sure that there are actually multiple of 2 total.
	fields := strings.Fields(value)
	if len(fields)%2 != 0 {
		return fmt.Errorf("%w `t=%v`", errSDPInvalidSyntax, fields)
	}

	for i := 0; i < len(fields); i += 2 {
		var timeZone sdp.TimeZone

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

func unmarshalSessionEncryptionKey(s *sdp.SessionDescription, value string) error {
	encryptionKey := sdp.EncryptionKey(value)
	s.EncryptionKey = &encryptionKey
	return nil
}

func unmarshalSessionAttribute(s *sdp.SessionDescription, value string) error {
	i := strings.IndexRune(value, ':')
	var a sdp.Attribute
	if i > 0 {
		a = sdp.NewAttribute(value[:i], value[i+1:])
	} else {
		a = sdp.NewPropertyAttribute(value)
	}

	s.Attributes = append(s.Attributes, a)
	return nil
}

func unmarshalTiming(s *sdp.SessionDescription, value string) error {
	if value == "now-" {
		// special case for some FLIR cameras with invalid timing element
		value = "0 0"
	}
	fields := strings.Fields(value)
	if len(fields) < 2 {
		return fmt.Errorf("%w `t=%v`", errSDPInvalidSyntax, fields)
	}

	td := sdp.TimeDescription{}

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

func unmarshalRepeatTimes(s *sdp.SessionDescription, value string) error {
	fields := strings.Fields(value)
	if len(fields) < 2 {
		return fmt.Errorf("%w `r=%v`", errSDPInvalidSyntax, value)
	}

	latestTimeDesc := &s.TimeDescriptions[len(s.TimeDescriptions)-1]

	newRepeatTime := sdp.RepeatTime{}
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
		var offset int64
		offset, err = parseTimeUnits(fields[i])
		if err != nil {
			return fmt.Errorf("%w `%v`", errSDPInvalidValue, fields)
		}
		newRepeatTime.Offsets = append(newRepeatTime.Offsets, offset)
	}
	latestTimeDesc.RepeatTimes = append(latestTimeDesc.RepeatTimes, newRepeatTime)

	return nil
}

func unmarshalMediaDescription(s *sdp.SessionDescription, value string) error {
	fields := strings.Fields(value)
	if len(fields) < 4 {
		return fmt.Errorf("%w `m=%v`", errSDPInvalidSyntax, fields)
	}

	newMediaDesc := &sdp.MediaDescription{}

	// <media>
	// Set according to currently registered with IANA
	// https://tools.ietf.org/html/rfc4566#section-5.14
	if fields[0] != "video" &&
		fields[0] != "audio" &&
		fields[0] != "application" &&
		!strings.HasPrefix(fields[0], "application/") &&
		fields[0] != "metadata" &&
		fields[0] != "meta" &&
		fields[0] != "text" {
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
		var portRange int
		portRange, err = strconv.Atoi(parts[1])
		if err != nil {
			return fmt.Errorf("%w `%v`", errSDPInvalidValue, parts)
		}
		newMediaDesc.MediaName.Port.Range = &portRange
	}

	// <proto>
	// Set according to currently registered with IANA
	// https://tools.ietf.org/html/rfc4566#section-5.14
	for proto := range strings.SplitSeq(fields[2], "/") {
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

func unmarshalMediaTitle(s *sdp.SessionDescription, value string) error {
	latestMediaDesc := s.MediaDescriptions[len(s.MediaDescriptions)-1]
	mediaTitle := sdp.Information(value)
	latestMediaDesc.MediaTitle = &mediaTitle
	return nil
}

func unmarshalMediaConnectionInformation(s *sdp.SessionDescription, value string) error {
	if strings.HasPrefix(value, "SM ") {
		return nil
	}

	latestMediaDesc := s.MediaDescriptions[len(s.MediaDescriptions)-1]
	var err error
	latestMediaDesc.ConnectionInformation, err = unmarshalConnectionInformation(value)
	if err != nil {
		return fmt.Errorf("%w `c=%v`", errSDPInvalidSyntax, value)
	}

	return nil
}

func unmarshalMediaBandwidth(s *sdp.SessionDescription, value string) error {
	latestMediaDesc := s.MediaDescriptions[len(s.MediaDescriptions)-1]
	bandwidth, err := unmarshalBandwidth(s, value)
	if err != nil {
		return fmt.Errorf("%w `b=%v`", errSDPInvalidSyntax, value)
	}
	latestMediaDesc.Bandwidth = append(latestMediaDesc.Bandwidth, *bandwidth)
	return nil
}

func unmarshalMediaEncryptionKey(s *sdp.SessionDescription, value string) error {
	latestMediaDesc := s.MediaDescriptions[len(s.MediaDescriptions)-1]
	encryptionKey := sdp.EncryptionKey(value)
	latestMediaDesc.EncryptionKey = &encryptionKey
	return nil
}

func unmarshalMediaAttribute(s *sdp.SessionDescription, value string) error {
	i := strings.IndexRune(value, ':')
	var a sdp.Attribute
	if i > 0 {
		a = sdp.NewAttribute(value[:i], value[i+1:])
	} else {
		a = sdp.NewPropertyAttribute(value)
	}

	latestMediaDesc := s.MediaDescriptions[len(s.MediaDescriptions)-1]
	latestMediaDesc.Attributes = append(latestMediaDesc.Attributes, a)
	return nil
}

type unmarshalState int

const (
	stateInitial unmarshalState = iota
	stateSession
	stateMedia
	stateTimeDescription
)

func unmarshalSession(s *sdp.SessionDescription, state *unmarshalState, key byte, val string) error {
	switch key {
	case 'o':
		err := unmarshalOrigin(s, val)
		if err != nil {
			return err
		}

	case 's':
		err := unmarshalSessionName(s, val)
		if err != nil {
			return err
		}

	case 'i':
		err := unmarshalSessionInformation(s, val)
		if err != nil {
			return err
		}

	case 'u':
		err := unmarshalURI(s, val)
		if err != nil {
			return err
		}

	case 'e':
		err := unmarshalEmail(s, val)
		if err != nil {
			return err
		}

	case 'p':
		err := unmarshalPhone(s, val)
		if err != nil {
			return err
		}

	case 'c':
		err := unmarshalSessionConnectionInformation(s, val)
		if err != nil {
			return err
		}

	case 'b':
		err := unmarshalSessionBandwidth(s, val)
		if err != nil {
			return err
		}

	case 'z':
		err := unmarshalTimeZones(s, val)
		if err != nil {
			return err
		}

	case 'k':
		err := unmarshalSessionEncryptionKey(s, val)
		if err != nil {
			return err
		}

	case 'a':
		err := unmarshalSessionAttribute(s, val)
		if err != nil {
			return err
		}

	case 't':
		err := unmarshalTiming(s, val)
		if err != nil {
			return err
		}
		*state = stateTimeDescription

	case 'm':
		err := unmarshalMediaDescription(s, val)
		if err != nil {
			return err
		}
		*state = stateMedia

	default:
		return fmt.Errorf("invalid key: %c", key)
	}

	return nil
}

func unmarshalMedia(s *sdp.SessionDescription, key byte, val string) error {
	switch key {
	case 'm':
		err := unmarshalMediaDescription(s, val)
		if err != nil {
			return err
		}

	case 'i':
		err := unmarshalMediaTitle(s, val)
		if err != nil {
			return err
		}

	case 'c':
		err := unmarshalMediaConnectionInformation(s, val)
		if err != nil {
			return err
		}

	case 'b':
		err := unmarshalMediaBandwidth(s, val)
		if err != nil {
			return err
		}

	case 'k':
		err := unmarshalMediaEncryptionKey(s, val)
		if err != nil {
			return err
		}

	case 'a':
		err := unmarshalMediaAttribute(s, val)
		if err != nil {
			return err
		}

	default:
		return fmt.Errorf("invalid key: %c", key)
	}

	return nil
}

// Unmarshal is a replacement for pion/sdp.Unmarshal, rewritten from scratch
// to provide compatibility with most RTSP implementations.
func Unmarshal(byts []byte) (*sdp.SessionDescription, error) {
	s := &sdp.SessionDescription{}
	str := string(byts)
	state := stateInitial

	for line := range strings.SplitSeq(strings.ReplaceAll(str, "\r", ""), "\n") {
		if line == "" {
			continue
		}

		if len(line) < 2 || line[1] != '=' {
			return nil, fmt.Errorf("invalid line: (%s)", line)
		}

		key := line[0]
		val := line[2:]

		switch state {
		case stateInitial:
			switch key {
			case 'v':
				err := unmarshalProtocolVersion(s, val)
				if err != nil {
					return nil, err
				}
				state = stateSession

			default:
				state = stateSession
				err := unmarshalSession(s, &state, key, val)
				if err != nil {
					return nil, err
				}
			}

		case stateSession:
			err := unmarshalSession(s, &state, key, val)
			if err != nil {
				return nil, err
			}

		case stateMedia:
			err := unmarshalMedia(s, key, val)
			if err != nil {
				return nil, err
			}

		case stateTimeDescription:
			switch key {
			case 'r':
				err := unmarshalRepeatTimes(s, val)
				if err != nil {
					return nil, err
				}

			default:
				state = stateSession
				err := unmarshalSession(s, &state, key, val)
				if err != nil {
					return nil, err
				}
			}
		}
	}

	return s, nil
}
