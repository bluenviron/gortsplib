package gortsplib

import (
	"strconv"
	"strings"
)

type TransportHeader map[string]struct{}

func NewTransportHeader(in string) TransportHeader {
	th := make(map[string]struct{})
	for _, t := range strings.Split(in, ";") {
		th[t] = struct{}{}
	}
	return th
}

func (th TransportHeader) GetValue(key string) string {
	prefix := key + "="
	for t := range th {
		if strings.HasPrefix(t, prefix) {
			return t[len(prefix):]
		}
	}
	return ""
}

func (th TransportHeader) GetPorts(key string) (int, int) {
	val := th.GetValue(key)
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
