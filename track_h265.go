package gortsplib

import (
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"sync"

	psdp "github.com/pion/sdp/v3"
)

// TrackH265 is a H265 track.
type TrackH265 struct {
	PayloadType uint8
	VPS         []byte
	SPS         []byte
	PPS         []byte

	trackBase
	mutex sync.RWMutex
}

func newTrackH265FromMediaDescription(
	control string,
	payloadType uint8,
	md *psdp.MediaDescription,
) (*TrackH265, error) {
	t := &TrackH265{
		PayloadType: payloadType,
		trackBase: trackBase{
			control: control,
		},
	}

	t.fillParamsFromMediaDescription(md)

	return t, nil
}

func (t *TrackH265) fillParamsFromMediaDescription(md *psdp.MediaDescription) error {
	v, ok := md.Attribute("fmtp")
	if !ok {
		return fmt.Errorf("fmtp attribute is missing")
	}

	tmp := strings.SplitN(v, " ", 2)
	if len(tmp) != 2 {
		return fmt.Errorf("invalid fmtp attribute (%v)", v)
	}

	for _, kv := range strings.Split(tmp[1], ";") {
		kv = strings.Trim(kv, " ")

		if len(kv) == 0 {
			continue
		}

		tmp := strings.SplitN(kv, "=", 2)
		if len(tmp) != 2 {
			return fmt.Errorf("invalid fmtp attribute (%v)", v)
		}

		switch tmp[0] {
		case "sprop-vps":
			var err error
			t.VPS, err = base64.StdEncoding.DecodeString(tmp[1])
			if err != nil {
				return fmt.Errorf("invalid sprop-vps (%v)", v)
			}

		case "sprop-sps":
			var err error
			t.SPS, err = base64.StdEncoding.DecodeString(tmp[1])
			if err != nil {
				return fmt.Errorf("invalid sprop-sps (%v)", v)
			}

		case "sprop-pps":
			var err error
			t.PPS, err = base64.StdEncoding.DecodeString(tmp[1])
			if err != nil {
				return fmt.Errorf("invalid sprop-pps (%v)", v)
			}
		}
	}

	return nil
}

// ClockRate returns the track clock rate.
func (t *TrackH265) ClockRate() int {
	return 90000
}

// MediaDescription returns the track media description in SDP format.
func (t *TrackH265) MediaDescription() *psdp.MediaDescription {
	t.mutex.RLock()
	defer t.mutex.RUnlock()

	typ := strconv.FormatInt(int64(t.PayloadType), 10)

	fmtp := typ

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
	if tmp != nil {
		fmtp += " " + strings.Join(tmp, "; ")
	}

	return &psdp.MediaDescription{
		MediaName: psdp.MediaName{
			Media:   "video",
			Protos:  []string{"RTP", "AVP"},
			Formats: []string{typ},
		},
		Attributes: []psdp.Attribute{
			{
				Key:   "rtpmap",
				Value: typ + " H265/90000",
			},
			{
				Key:   "fmtp",
				Value: fmtp,
			},
			{
				Key:   "control",
				Value: t.control,
			},
		},
	}
}

func (t *TrackH265) clone() Track {
	return &TrackH265{
		PayloadType: t.PayloadType,
		VPS:         t.VPS,
		SPS:         t.SPS,
		PPS:         t.PPS,
		trackBase:   t.trackBase,
	}
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
