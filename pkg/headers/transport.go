package headers

import (
	"encoding/binary"
	"encoding/hex"
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

// Transport is a Transport header.
type Transport struct {
	// protocol of the stream
	Protocol base.StreamProtocol

	// (optional) delivery method of the stream
	Delivery *base.StreamDelivery

	// (optional) destination
	Destination *string

	// (optional) interleaved frame ids
	InterleavedIDs *[2]int

	// (optional) TTL
	TTL *uint

	// (optional) ports
	Ports *[2]int

	// (optional) client ports
	ClientPorts *[2]int

	// (optional) server ports
	ServerPorts *[2]int

	// (optional) SSRC of the packets of the stream
	SSRC *uint32

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

	v0 := v[0]

	kvs, err := keyValParse(v0, ';')
	if err != nil {
		return err
	}

	protocolFound := false

	for k, rv := range kvs {
		v := rv

		switch k {
		case "RTP/AVP", "RTP/AVP/UDP":
			h.Protocol = base.StreamProtocolUDP
			protocolFound = true

		case "RTP/AVP/TCP":
			h.Protocol = base.StreamProtocolTCP
			protocolFound = true

		case "unicast":
			v := base.StreamDeliveryUnicast
			h.Delivery = &v

		case "multicast":
			v := base.StreamDeliveryMulticast
			h.Delivery = &v

		case "destination":
			h.Destination = &v

		case "interleaved":
			ports, err := parsePorts(v)
			if err != nil {
				return err
			}
			h.InterleavedIDs = ports

		case "ttl":
			tmp, err := strconv.ParseUint(v, 10, 64)
			if err != nil {
				return err
			}
			vu := uint(tmp)
			h.TTL = &vu

		case "port":
			ports, err := parsePorts(v)
			if err != nil {
				return err
			}
			h.Ports = ports

		case "client_port":
			ports, err := parsePorts(v)
			if err != nil {
				return err
			}
			h.ClientPorts = ports

		case "server_port":
			ports, err := parsePorts(v)
			if err != nil {
				return err
			}
			h.ServerPorts = ports

		case "ssrc":
			if (len(v) % 2) != 0 {
				v = "0" + v
			}
			tmp, err := hex.DecodeString(v)
			if err != nil {
				return err
			}

			if len(tmp) != 4 {
				return fmt.Errorf("invalid SSRC")
			}

			v := binary.BigEndian.Uint32(tmp)
			h.SSRC = &v

		case "mode":
			str := strings.ToLower(v)
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

		default:
			// ignore non-standard keys
		}
	}

	if !protocolFound {
		return fmt.Errorf("protocol not found (%v)", v[0])
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

	if h.InterleavedIDs != nil {
		ports := *h.InterleavedIDs
		rets = append(rets, "interleaved="+strconv.FormatInt(int64(ports[0]), 10)+"-"+strconv.FormatInt(int64(ports[1]), 10))
	}

	if h.ClientPorts != nil {
		ports := *h.ClientPorts
		rets = append(rets, "client_port="+strconv.FormatInt(int64(ports[0]), 10)+"-"+strconv.FormatInt(int64(ports[1]), 10))
	}

	if h.ServerPorts != nil {
		ports := *h.ServerPorts
		rets = append(rets, "server_port="+strconv.FormatInt(int64(ports[0]), 10)+"-"+strconv.FormatInt(int64(ports[1]), 10))
	}

	if h.SSRC != nil {
		tmp := make([]byte, 4)
		binary.BigEndian.PutUint32(tmp, *h.SSRC)
		rets = append(rets, "ssrc="+strings.ToUpper(hex.EncodeToString(tmp)))
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
