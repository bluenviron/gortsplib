package gortsplib

import (
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/pion/rtp"

	"github.com/aler9/gortsplib/pkg/rtpcodecs/rtph265"
)

// TrackH265 is a H265 track.
type TrackH265 struct {
	PayloadType uint8
	VPS         []byte
	SPS         []byte
	PPS         []byte
	MaxDONDiff  int

	mutex sync.RWMutex
}

// String returns a description of the track.
func (t *TrackH265) String() string {
	return "H265"
}

// ClockRate returns the clock rate.
func (t *TrackH265) ClockRate() int {
	return 90000
}

// GetPayloadType returns the payload type.
func (t *TrackH265) GetPayloadType() uint8 {
	return t.PayloadType
}

func (t *TrackH265) unmarshal(payloadType uint8, clock string, codec string, rtpmap string, fmtp string) error {
	t.PayloadType = payloadType

	if fmtp == "" {
		return nil // do not return any error
	}

	for _, kv := range strings.Split(fmtp, ";") {
		kv = strings.Trim(kv, " ")

		if len(kv) == 0 {
			continue
		}

		tmp := strings.SplitN(kv, "=", 2)
		if len(tmp) != 2 {
			return fmt.Errorf("invalid fmtp attribute (%v)", fmtp)
		}

		switch tmp[0] {
		case "sprop-vps":
			var err error
			t.VPS, err = base64.StdEncoding.DecodeString(tmp[1])
			if err != nil {
				return fmt.Errorf("invalid sprop-vps (%v)", fmtp)
			}

		case "sprop-sps":
			var err error
			t.SPS, err = base64.StdEncoding.DecodeString(tmp[1])
			if err != nil {
				return fmt.Errorf("invalid sprop-sps (%v)", fmtp)
			}

		case "sprop-pps":
			var err error
			t.PPS, err = base64.StdEncoding.DecodeString(tmp[1])
			if err != nil {
				return fmt.Errorf("invalid sprop-pps (%v)", fmtp)
			}

		case "sprop-max-don-diff":
			tmp, err := strconv.ParseInt(tmp[1], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid sprop-max-don-diff (%v)", fmtp)
			}
			t.MaxDONDiff = int(tmp)
		}
	}

	return nil
}

func (t *TrackH265) marshal() (string, string) {
	t.mutex.RLock()
	defer t.mutex.RUnlock()

	var tmp []string
	if t.VPS != nil {
		tmp = append(tmp, "sprop-vps="+base64.StdEncoding.EncodeToString(t.VPS))
	}
	if t.SPS != nil {
		tmp = append(tmp, "sprop-sps="+base64.StdEncoding.EncodeToString(t.SPS))
	}
	if t.PPS != nil {
		tmp = append(tmp, "sprop-pps="+base64.StdEncoding.EncodeToString(t.PPS))
	}
	if t.MaxDONDiff != 0 {
		tmp = append(tmp, "sprop-max-don-diff="+strconv.FormatInt(int64(t.MaxDONDiff), 10))
	}
	var fmtp string
	if tmp != nil {
		fmtp = strings.Join(tmp, "; ")
	}

	return "H265/90000", fmtp
}

func (t *TrackH265) clone() Track {
	return &TrackH265{
		PayloadType: t.PayloadType,
		VPS:         t.VPS,
		SPS:         t.SPS,
		PPS:         t.PPS,
		MaxDONDiff:  t.MaxDONDiff,
	}
}

func (t *TrackH265) ptsEqualsDTS(*rtp.Packet) bool {
	return true
}

// CreateDecoder creates a decoder able to decode the content of the track.
func (t *TrackH265) CreateDecoder() *rtph265.Decoder {
	d := &rtph265.Decoder{
		MaxDONDiff: t.MaxDONDiff,
	}
	d.Init()
	return d
}

// CreateEncoder creates an encoder able to encode the content of the track.
func (t *TrackH265) CreateEncoder() *rtph265.Encoder {
	e := &rtph265.Encoder{
		PayloadType: t.PayloadType,
		MaxDONDiff:  t.MaxDONDiff,
	}
	e.Init()
	return e
}

// SafeVPS returns the track VPS.
func (t *TrackH265) SafeVPS() []byte {
	t.mutex.RLock()
	defer t.mutex.RUnlock()
	return t.VPS
}

// SafeSPS returns the track SPS.
func (t *TrackH265) SafeSPS() []byte {
	t.mutex.RLock()
	defer t.mutex.RUnlock()
	return t.SPS
}

// SafePPS returns the track PPS.
func (t *TrackH265) SafePPS() []byte {
	t.mutex.RLock()
	defer t.mutex.RUnlock()
	return t.PPS
}

// SafeSetVPS sets the track VPS.
func (t *TrackH265) SafeSetVPS(v []byte) {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	t.VPS = v
}

// SafeSetSPS sets the track SPS.
func (t *TrackH265) SafeSetSPS(v []byte) {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	t.SPS = v
}

// SafeSetPPS sets the track PPS.
func (t *TrackH265) SafeSetPPS(v []byte) {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	t.PPS = v
}
