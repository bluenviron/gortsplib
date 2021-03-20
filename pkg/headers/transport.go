package headers

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/aler9/gortsplib/pkg/base"
)

// TransportMode is a transport mode.
type TransportMode int

const (
	// TransportModePlay is the "play" transport mode
	TransportModePlay TransportMode = iota

	// TransportModeRecord is the "record" transport mode
	TransportModeRecord
)

// String implements fmt.Stringer.
func (tm TransportMode) String() string {
	switch tm {
	case TransportModePlay:
		return "play"

	case TransportModeRecord:
		return "record"
	}
	return "unknown"
}

// Transport is a Transport header.
type Transport struct {
	// protocol of the stream
	Protocol base.StreamProtocol

	// (optional) delivery method of the stream
	Delivery *base.StreamDelivery

	// (optional) destination
	Destination *string

	// (optional) TTL
	TTL *uint

	// (optional) ports
	Ports *[2]int

	// (optional) client ports
	ClientPorts *[2]int

	// (optional) server ports
	ServerPorts *[2]int

	// (optional) interleaved frame ids
	InterleavedIds *[2]int

	// (optional) mode
	Mode *TransportMode
}

func parsePorts(val string) (*[2]int, error) {
	ports := strings.Split(val, "-")
	if len(ports) == 2 {
		port1, err := strconv.ParseInt(ports[0], 10, 64)
		if err != nil {
			return &[2]int{0, 0}, fmt.Errorf("invalid ports (%v)", val)
		}

		port2, err := strconv.ParseInt(ports[1], 10, 64)
		if err != nil {
			return &[2]int{0, 0}, fmt.Errorf("invalid ports (%v)", val)
		}

		return &[2]int{int(port1), int(port2)}, nil
	}

	if len(ports) == 1 {
		port1, err := strconv.ParseInt(ports[0], 10, 64)
		if err != nil {
			return &[2]int{0, 0}, fmt.Errorf("invalid ports (%v)", val)
		}

		return &[2]int{int(port1), int(port1 + 1)}, nil
	}

	return &[2]int{0, 0}, fmt.Errorf("invalid ports (%v)", val)
}

// Read decodes a Transport header.
func (h *Transport) Read(v base.HeaderValue) error {
	if len(v) == 0 {
		return fmt.Errorf("value not provided")
	}

	if len(v) > 1 {
		return fmt.Errorf("value provided multiple times (%v)", v)
	}

	parts := strings.Split(v[0], ";")
	if len(parts) == 0 {
		return fmt.Errorf("invalid value (%v)", v)
	}

	switch parts[0] {
	case "RTP/AVP", "RTP/AVP/UDP":
		h.Protocol = base.StreamProtocolUDP

	case "RTP/AVP/TCP":
		h.Protocol = base.StreamProtocolTCP

	default:
		return fmt.Errorf("invalid protocol (%v)", v)
	}
	parts = parts[1:]

	switch parts[0] {
	case "unicast":
		v := base.StreamDeliveryUnicast
		h.Delivery = &v
		parts = parts[1:]

	case "multicast":
		v := base.StreamDeliveryMulticast
		h.Delivery = &v
		parts = parts[1:]

		// cast is optional, do not return any error
	}

	for _, t := range parts {
		if strings.HasPrefix(t, "destination=") {
			v := t[len("destination="):]
			h.Destination = &v

		} else if strings.HasPrefix(t, "ttl=") {
			v, err := strconv.ParseUint(t[len("ttl="):], 10, 64)
			if err != nil {
				return err
			}
			vu := uint(v)
			h.TTL = &vu

		} else if strings.HasPrefix(t, "port=") {
			ports, err := parsePorts(t[len("port="):])
			if err != nil {
				return err
			}
			h.Ports = ports

		} else if strings.HasPrefix(t, "client_port=") {
			ports, err := parsePorts(t[len("client_port="):])
			if err != nil {
				return err
			}
			h.ClientPorts = ports

		} else if strings.HasPrefix(t, "server_port=") {
			ports, err := parsePorts(t[len("server_port="):])
			if err != nil {
				return err
			}
			h.ServerPorts = ports

		} else if strings.HasPrefix(t, "interleaved=") {
			ports, err := parsePorts(t[len("interleaved="):])
			if err != nil {
				return err
			}
			h.InterleavedIds = ports

		} else if strings.HasPrefix(t, "mode=") {
			str := strings.ToLower(t[len("mode="):])
			str = strings.TrimPrefix(str, "\"")
			str = strings.TrimSuffix(str, "\"")

			switch str {
			case "play":
				v := TransportModePlay
				h.Mode = &v

				// receive is an old alias for record, used by ffmpeg with the
				// -listen flag, and by Darwin Streaming Server
			case "record", "receive":
				v := TransportModeRecord
				h.Mode = &v

			default:
				return fmt.Errorf("invalid transport mode: '%s'", str)
			}
		}

		// ignore non-standard keys
	}

	return nil
}

// Write encodes a Transport header
func (h Transport) Write() base.HeaderValue {
	var rets []string

	if h.Protocol == base.StreamProtocolUDP {
		rets = append(rets, "RTP/AVP")
	} else {
		rets = append(rets, "RTP/AVP/TCP")
	}

	if h.Delivery != nil {
		if *h.Delivery == base.StreamDeliveryUnicast {
			rets = append(rets, "unicast")
		} else {
			rets = append(rets, "multicast")
		}
	}

	if h.ClientPorts != nil {
		ports := *h.ClientPorts
		rets = append(rets, "client_port="+strconv.FormatInt(int64(ports[0]), 10)+"-"+strconv.FormatInt(int64(ports[1]), 10))
	}

	if h.ServerPorts != nil {
		ports := *h.ServerPorts
		rets = append(rets, "server_port="+strconv.FormatInt(int64(ports[0]), 10)+"-"+strconv.FormatInt(int64(ports[1]), 10))
	}

	if h.InterleavedIds != nil {
		ports := *h.InterleavedIds
		rets = append(rets, "interleaved="+strconv.FormatInt(int64(ports[0]), 10)+"-"+strconv.FormatInt(int64(ports[1]), 10))
	}

	if h.Mode != nil {
		if *h.Mode == TransportModePlay {
			rets = append(rets, "mode=play")
		} else {
			rets = append(rets, "mode=record")
		}
	}

	return base.HeaderValue{strings.Join(rets, ";")}
}
