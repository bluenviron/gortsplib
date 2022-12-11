package headers

import (
	"encoding/hex"
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/aler9/gortsplib/v2/pkg/base"
)

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

// Transport is a Transport header.
type Transport struct {
	// protocol of the stream
	Protocol TransportProtocol

	// (optional) delivery method of the stream
	Delivery *TransportDelivery

	// (optional) Source IP
	Source *net.IP

	// (optional) destination IP
	Destination *net.IP

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

// Unmarshal decodes a Transport header.
func (h *Transport) Unmarshal(v base.HeaderValue) error {
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
			h.Protocol = TransportProtocolUDP
			protocolFound = true

		case "RTP/AVP/TCP":
			h.Protocol = TransportProtocolTCP
			protocolFound = true

		case "unicast":
			v := TransportDeliveryUnicast
			h.Delivery = &v

		case "multicast":
			v := TransportDeliveryMulticast
			h.Delivery = &v

		case "source":
			if v != "" {
				ip := net.ParseIP(v)
				if ip == nil {
					addrs, err := net.LookupHost(v)
					if err != nil {
						return fmt.Errorf("invalid source (%v)", v)
					}
					ip = net.ParseIP(addrs[0])
					if ip == nil {
						return fmt.Errorf("invalid source (%v)", v)
					}
				}
				h.Source = &ip
			}

		case "destination":
			if v != "" {
				ip := net.ParseIP(v)
				if ip == nil {
					return fmt.Errorf("invalid destination (%v)", v)
				}
				h.Destination = &ip
			}

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
			v = strings.TrimLeft(v, " ")

			if (len(v) % 2) != 0 {
				v = "0" + v
			}

			if tmp, err := hex.DecodeString(v); err == nil && len(tmp) <= 4 {
				var ssrc [4]byte
				copy(ssrc[4-len(tmp):], tmp)
				v := uint32(ssrc[0])<<24 | uint32(ssrc[1])<<16 | uint32(ssrc[2])<<8 | uint32(ssrc[3])
				h.SSRC = &v
			}

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

// Marshal encodes a Transport header.
func (h Transport) Marshal() base.HeaderValue {
	var rets []string

	if h.Protocol == TransportProtocolUDP {
		rets = append(rets, "RTP/AVP")
	} else {
		rets = append(rets, "RTP/AVP/TCP")
	}

	if h.Delivery != nil {
		if *h.Delivery == TransportDeliveryUnicast {
			rets = append(rets, "unicast")
		} else {
			rets = append(rets, "multicast")
		}
	}

	if h.Source != nil {
		rets = append(rets, "source="+h.Source.String())
	}

	if h.Destination != nil {
		rets = append(rets, "destination="+h.Destination.String())
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
		if *h.Mode == TransportModePlay {
			rets = append(rets, "mode=play")
		} else {
			rets = append(rets, "mode=record")
		}
	}

	return base.HeaderValue{strings.Join(rets, ";")}
}

// Transports is a Transport header with multiple transports.
type Transports []Transport

// Unmarshal decodes a Transport header.
func (ts *Transports) Unmarshal(v base.HeaderValue) error {
	if len(v) == 0 {
		return fmt.Errorf("value not provided")
	}

	if len(v) > 1 {
		return fmt.Errorf("value provided multiple times (%v)", v)
	}

	v0 := v[0]
	transports := strings.Split(v0, ",") // , separated per RFC2326 section 12.39
	*ts = make([]Transport, len(transports))

	for i, transport := range transports {
		var tr Transport
		err := tr.Unmarshal(base.HeaderValue{strings.TrimLeft(transport, " ")})
		if err != nil {
			return err
		}
		(*ts)[i] = tr
	}

	return nil
}

// Marshal encodes a Transport header.
func (ts Transports) Marshal() base.HeaderValue {
	vals := make([]string, len(ts))

	for i, th := range ts {
		vals[i] = th.Marshal()[0]
	}

	return base.HeaderValue{strings.Join(vals, ",")}
}
