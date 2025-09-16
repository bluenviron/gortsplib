package headers

import (
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	"github.com/bluenviron/gortsplib/v5/pkg/base"
)

func parsePorts(val string) (*[2]int, error) {
	ports := strings.Split(val, "-")
	if len(ports) == 2 {
		port1, err := strconv.ParseUint(ports[0], 10, 31)
		if err != nil {
			return &[2]int{0, 0}, fmt.Errorf("invalid ports (%v)", val)
		}

		port2, err := strconv.ParseUint(ports[1], 10, 31)
		if err != nil {
			return &[2]int{0, 0}, fmt.Errorf("invalid ports (%v)", val)
		}

		return &[2]int{int(port1), int(port2)}, nil
	}

	if len(ports) == 1 {
		port1, err := strconv.ParseUint(ports[0], 10, 31)
		if err != nil {
			return &[2]int{0, 0}, fmt.Errorf("invalid ports (%v)", val)
		}

		return &[2]int{int(port1), int(port1 + 1)}, nil
	}

	return &[2]int{0, 0}, fmt.Errorf("invalid ports (%v)", val)
}

// TransportProfile is a transport profile.
type TransportProfile int

// transport profiles.
const (
	TransportProfileAVP TransportProfile = iota
	TransportProfileSAVP
)

// TransportProtocol is a transport protocol.
type TransportProtocol int

// transport protocols.
const (
	TransportProtocolUDP TransportProtocol = iota
	TransportProtocolTCP
)

// TransportDelivery is a delivery method.
type TransportDelivery int

// transport delivery methods.
const (
	TransportDeliveryUnicast TransportDelivery = iota
	TransportDeliveryMulticast
)

// TransportMode is a transport mode.
type TransportMode int

const (
	// TransportModePlay is the "play" transport mode
	TransportModePlay TransportMode = iota

	// TransportModeRecord is the "record" transport mode
	TransportModeRecord
)

func (m *TransportMode) unmarshal(v string) error {
	str := strings.ToLower(v)

	switch str {
	case "play":
		*m = TransportModePlay
		return nil

		// receive is an old alias for record, used by ffmpeg with the
		// -listen flag, and by Darwin Streaming Server
	case "record", "receive":
		*m = TransportModeRecord
		return nil

	default:
		return fmt.Errorf("invalid transport mode: '%s'", str)
	}
}

// String implements fmt.Stringer.
func (m TransportMode) String() string {
	if m == TransportModePlay {
		return "play"
	}
	return "record"
}

// Transport is a Transport header.
type Transport struct {
	// profile.
	Profile TransportProfile

	// protocol.
	Protocol TransportProtocol

	// (optional) delivery method.
	Delivery *TransportDelivery

	// (optional) Source IP/host.
	Source2 *string

	// (optional) destination IP/host.
	Destination2 *string

	// (optional) interleaved frame IDs.
	InterleavedIDs *[2]int

	// (optional) TTL.
	TTL *uint

	// (optional) ports.
	Ports *[2]int

	// (optional) client ports.
	ClientPorts *[2]int

	// (optional) server ports.
	ServerPorts *[2]int

	// (optional) SSRC of packets.
	SSRC *uint32

	// (optional) mode.
	Mode *TransportMode
}

// Unmarshal decodes a Transport header.
func (h *Transport) Unmarshal(v base.HeaderValue) error {
	if len(v) == 0 {
		return fmt.Errorf("value not provided")
	}

	if len(v) > 1 {
		return fmt.Errorf("value provided multiple times (%v)", v)
	}

	kvs, err := keyValParse(v[0], ';')
	if err != nil {
		return err
	}

	profileFound := false

	for k, rv := range kvs {
		v := rv

		switch k {
		case "RTP/AVP", "RTP/AVP/UDP":
			h.Profile = TransportProfileAVP
			h.Protocol = TransportProtocolUDP
			profileFound = true

		case "RTP/AVP/TCP":
			h.Profile = TransportProfileAVP
			h.Protocol = TransportProtocolTCP
			profileFound = true

		case "RTP/SAVP", "RTP/SAVP/UDP":
			h.Protocol = TransportProtocolUDP
			h.Profile = TransportProfileSAVP
			profileFound = true

		case "RTP/SAVP/TCP":
			h.Profile = TransportProfileSAVP
			h.Protocol = TransportProtocolTCP
			profileFound = true

		case "unicast":
			v := TransportDeliveryUnicast
			h.Delivery = &v

		case "multicast":
			v := TransportDeliveryMulticast
			h.Delivery = &v

		case "source":
			if v != "" {
				h.Source2 = &v
			}

		case "destination":
			if v != "" {
				h.Destination2 = &v
			}

		case "interleaved":
			ports, err2 := parsePorts(v)
			if err2 != nil {
				return err2
			}
			h.InterleavedIDs = ports

		case "ttl":
			tmp, err2 := strconv.ParseUint(v, 10, 32)
			if err2 != nil {
				return err2
			}
			vu := uint(tmp)
			h.TTL = &vu

		case "port":
			ports, err2 := parsePorts(v)
			if err2 != nil {
				return err2
			}
			h.Ports = ports

		case "client_port":
			ports, err2 := parsePorts(v)
			if err2 != nil {
				return err2
			}
			h.ClientPorts = ports

		case "server_port":
			ports, err2 := parsePorts(v)
			if err2 != nil {
				return err2
			}
			h.ServerPorts = ports

		case "ssrc":
			v = strings.TrimLeft(v, " ")

			if (len(v) % 2) != 0 {
				v = "0" + v
			}

			if tmp, err2 := hex.DecodeString(v); err2 == nil && len(tmp) <= 4 {
				var ssrc [4]byte
				copy(ssrc[4-len(tmp):], tmp)
				v := uint32(ssrc[0])<<24 | uint32(ssrc[1])<<16 | uint32(ssrc[2])<<8 | uint32(ssrc[3])
				h.SSRC = &v
			}

		case "mode":
			var m TransportMode
			err = m.unmarshal(v)
			if err != nil {
				return err
			}
			h.Mode = &m

		default:
			// ignore non-standard keys
		}
	}

	if !profileFound {
		return fmt.Errorf("profile is missing: %v", v[0])
	}

	return nil
}

// Marshal encodes a Transport header.
func (h Transport) Marshal() base.HeaderValue {
	var rets []string

	var profile string

	switch {
	case h.Protocol == TransportProtocolUDP && h.Profile == TransportProfileAVP:
		profile = "RTP/AVP"
	case h.Protocol == TransportProtocolTCP && h.Profile == TransportProfileAVP:
		profile = "RTP/AVP/TCP"
	case h.Protocol == TransportProtocolUDP && h.Profile == TransportProfileSAVP:
		profile = "RTP/SAVP"
	case h.Protocol == TransportProtocolTCP && h.Profile == TransportProfileSAVP:
		profile = "RTP/SAVP/TCP"
	}

	rets = append(rets, profile)

	if h.Delivery != nil {
		var delivery string

		switch *h.Delivery {
		case TransportDeliveryUnicast:
			delivery = "unicast"

		case TransportDeliveryMulticast:
			delivery = "multicast"
		}

		rets = append(rets, delivery)
	}

	if h.Source2 != nil {
		rets = append(rets, "source="+*h.Source2)
	}

	if h.Destination2 != nil {
		rets = append(rets, "destination="+*h.Destination2)
	}

	if h.InterleavedIDs != nil {
		rets = append(rets, "interleaved="+strconv.FormatInt(int64(h.InterleavedIDs[0]), 10)+
			"-"+strconv.FormatInt(int64(h.InterleavedIDs[1]), 10))
	}

	if h.Ports != nil {
		rets = append(rets, "port="+strconv.FormatInt(int64(h.Ports[0]), 10)+
			"-"+strconv.FormatInt(int64(h.Ports[1]), 10))
	}

	if h.TTL != nil {
		rets = append(rets, "ttl="+strconv.FormatUint(uint64(*h.TTL), 10))
	}

	if h.ClientPorts != nil {
		rets = append(rets, "client_port="+strconv.FormatInt(int64(h.ClientPorts[0]), 10)+
			"-"+strconv.FormatInt(int64(h.ClientPorts[1]), 10))
	}

	if h.ServerPorts != nil {
		rets = append(rets, "server_port="+strconv.FormatInt(int64(h.ServerPorts[0]), 10)+
			"-"+strconv.FormatInt(int64(h.ServerPorts[1]), 10))
	}

	if h.SSRC != nil {
		tmp := make([]byte, 4)
		tmp[0] = byte(*h.SSRC >> 24)
		tmp[1] = byte(*h.SSRC >> 16)
		tmp[2] = byte(*h.SSRC >> 8)
		tmp[3] = byte(*h.SSRC)
		rets = append(rets, "ssrc="+strings.ToUpper(hex.EncodeToString(tmp)))
	}

	if h.Mode != nil {
		rets = append(rets, "mode="+h.Mode.String())
	}

	return base.HeaderValue{strings.Join(rets, ";")}
}
