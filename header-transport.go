package gortsplib

import (
	"strconv"
	"strings"
)

// HeaderTransport is a Transport header.
type HeaderTransport map[string]struct{}

// ReadHeaderTransport parses a Transport header.
func ReadHeaderTransport(in string) HeaderTransport {
	ht := make(map[string]struct{})
	for _, t := range strings.Split(in, ";") {
		ht[t] = struct{}{}
	}
	return ht
}

// GetValue gets a value from the header.
func (ht HeaderTransport) GetValue(key string) string {
	prefix := key + "="
	for t := range ht {
		if strings.HasPrefix(t, prefix) {
			return t[len(prefix):]
		}
	}
	return ""
}

// GetPorts gets a given header value and parses its ports.
func (ht HeaderTransport) GetPorts(key string) (int, int) {
	val := ht.GetValue(key)
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
