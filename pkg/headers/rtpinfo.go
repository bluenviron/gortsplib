package headers

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/aler9/gortsplib/pkg/base"
)

// RTPInfoEntry is an entry of an RTP-Info header.
type RTPInfoEntry struct {
	URL            *base.URL
	SequenceNumber uint16
	RTPTime        uint32
}

// RTPInfo is a RTP-Info header.
type RTPInfo []*RTPInfoEntry

// ReadRTPInfo decodes a RTP-Info header.
func ReadRTPInfo(v base.HeaderValue) (*RTPInfo, error) {
	if len(v) == 0 {
		return nil, fmt.Errorf("value not provided")
	}

	if len(v) > 1 {
		return nil, fmt.Errorf("value provided multiple times (%v)", v)
	}

	h := &RTPInfo{}

	for _, tmp := range strings.Split(v[0], ",") {
		e := &RTPInfoEntry{}

		for _, kv := range strings.Split(tmp, ";") {
			tmp := strings.SplitN(kv, "=", 2)
			if len(tmp) != 2 {
				return nil, fmt.Errorf("unable to parse key-value (%v)", kv)
			}

			k, v := tmp[0], tmp[1]
			switch k {
			case "url":
				vu, err := base.ParseURL(v)
				if err != nil {
					return nil, err
				}
				e.URL = vu

			case "seq":
				vi, err := strconv.ParseUint(v, 10, 16)
				if err != nil {
					return nil, err
				}
				e.SequenceNumber = uint16(vi)

			case "rtptime":
				vi, err := strconv.ParseUint(v, 10, 32)
				if err != nil {
					return nil, err
				}
				e.RTPTime = uint32(vi)

			default:
				return nil, fmt.Errorf("invalid key: %v", k)
			}
		}

		*h = append(*h, e)
	}

	return h, nil
}

// Clone clones a RTPInfo.
func (h RTPInfo) Clone() *RTPInfo {
	nh := &RTPInfo{}
	for _, e := range h {
		*nh = append(*nh, &RTPInfoEntry{
			URL:            e.URL,
			SequenceNumber: e.SequenceNumber,
			RTPTime:        e.RTPTime,
		})
	}
	return nh
}

// Write encodes a RTP-Info header.
func (h RTPInfo) Write() base.HeaderValue {
	var rets []string

	for _, e := range h {
		rets = append(rets, "url="+e.URL.String()+
			";seq="+strconv.FormatUint(uint64(e.SequenceNumber), 10)+
			";rtptime="+strconv.FormatUint(uint64(e.RTPTime), 10))
	}

	return base.HeaderValue{strings.Join(rets, ",")}
}
