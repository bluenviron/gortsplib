package gortsplib

import (
	"fmt"
	"strconv"
	"strings"
)

// HeaderTransport is a Transport header.
type HeaderTransport map[string]struct{}

// ReadHeaderTransport parses a Transport header.
func ReadHeaderTransport(v HeaderValue) (HeaderTransport, error) {
	if len(v) == 0 {
		return nil, fmt.Errorf("value not provided")
	}

	if len(v) > 1 {
		return nil, fmt.Errorf("value provided multiple times (%v)", v)
	}

	ht := make(map[string]struct{})
	for _, t := range strings.Split(v[0], ";") {
		ht[t] = struct{}{}
	}

	return ht, nil
}

// IsUdp check whether the header contains the UDP protocol.
func (ht HeaderTransport) IsUdp() bool {
	if _, ok := ht["RTP/AVP"]; ok {
		return true
	}
	if _, ok := ht["RTP/AVP/UDP"]; ok {
		return true
	}
	return false
}

// IsTcp check whether the header contains the TCP protocol.
func (ht HeaderTransport) IsTcp() bool {
	_, ok := ht["RTP/AVP/TCP"]
	return ok
}

// Value gets a value from the header.
func (ht HeaderTransport) Value(key string) string {
	prefix := key + "="
	for t := range ht {
		if strings.HasPrefix(t, prefix) {
			return t[len(prefix):]
		}
	}
	return ""
}

// Ports gets a value from the header and parses its ports.
func (ht HeaderTransport) Ports(key string) (int, int) {
	val := ht.Value(key)
	if val == "" {
		return 0, 0
	}

	ports := strings.Split(val, "-")
	if len(ports) != 2 {
		return 0, 0
	}

	port1, err := strconv.ParseInt(ports[0], 10, 64)
	if err != nil {
		return 0, 0
	}

	port2, err := strconv.ParseInt(ports[1], 10, 64)
	if err != nil {
		return 0, 0
	}

	return int(port1), int(port2)
}
