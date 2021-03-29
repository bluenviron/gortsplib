package headers

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/aler9/gortsplib/pkg/base"
)

// RTPInfoEntry is an entry of a RTP-Info header.
type RTPInfoEntry struct {
	URL            string
	SequenceNumber *uint16
	Timestamp      *uint32
}

// RTPInfo is a RTP-Info header.
type RTPInfo []*RTPInfoEntry

// Read decodes a RTP-Info header.
func (h *RTPInfo) Read(v base.HeaderValue) error {
	if len(v) == 0 {
		return fmt.Errorf("value not provided")
	}

	if len(v) > 1 {
		return fmt.Errorf("value provided multiple times (%v)", v)
	}

	for _, tmp := range strings.Split(v[0], ",") {
		e := &RTPInfoEntry{}

		for _, kv := range strings.Split(tmp, ";") {
			tmp := strings.SplitN(kv, "=", 2)
			if len(tmp) != 2 {
				return fmt.Errorf("unable to parse key-value (%v)", kv)
			}

			k, v := tmp[0], tmp[1]
			switch k {
			case "url":
				e.URL = v

			case "seq":
				vi, err := strconv.ParseUint(v, 10, 16)
				if err != nil {
					return err
				}
				vi2 := uint16(vi)
				e.SequenceNumber = &vi2

			case "rtptime":
				vi, err := strconv.ParseUint(v, 10, 32)
				if err != nil {
					return err
				}
				vi2 := uint32(vi)
				e.Timestamp = &vi2

			default:
				return fmt.Errorf("invalid key: %v", k)
			}
		}

		if e.URL == "" {
			return fmt.Errorf("URL is missing")
		}

		*h = append(*h, e)
	}

	return nil
}

// Write encodes a RTP-Info header.
func (h RTPInfo) Write() base.HeaderValue {
	rets := make([]string, len(h))

	for i, e := range h {
		var tmp []string
		tmp = append(tmp, "url="+e.URL)

		if e.SequenceNumber != nil {
			tmp = append(tmp, "seq="+strconv.FormatUint(uint64(*e.SequenceNumber), 10))
		}

		if e.Timestamp != nil {
			tmp = append(tmp, "rtptime="+strconv.FormatUint(uint64(*e.Timestamp), 10))
		}

		rets[i] = strings.Join(tmp, ";")
	}

	return base.HeaderValue{strings.Join(rets, ",")}
}
