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
	trackBase
	payloadType uint8
	vps         []byte
	sps         []byte
	pps         []byte
	mutex       sync.RWMutex
}

// NewTrackH265 allocates a TrackH265.
func NewTrackH265(payloadType uint8, vps []byte, sps []byte, pps []byte) *TrackH265 {
	return &TrackH265{
		payloadType: payloadType,
		vps:         vps,
		sps:         sps,
		pps:         pps,
	}
}

func newTrackH265FromMediaDescription(
	control string,
	payloadType uint8,
	md *psdp.MediaDescription,
) (*TrackH265, error) {
	t := &TrackH265{
		trackBase: trackBase{
			control: control,
		},
		payloadType: payloadType,
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
			t.vps, err = base64.StdEncoding.DecodeString(tmp[1])
			if err != nil {
				return fmt.Errorf("invalid sprop-vps (%v)", v)
			}

		case "sprop-sps":
			var err error
			t.sps, err = base64.StdEncoding.DecodeString(tmp[1])
			if err != nil {
				return fmt.Errorf("invalid sprop-sps (%v)", v)
			}

		case "sprop-pps":
			var err error
			t.pps, err = base64.StdEncoding.DecodeString(tmp[1])
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

func (t *TrackH265) clone() Track {
	return &TrackH265{
		trackBase:   t.trackBase,
		payloadType: t.payloadType,
		vps:         t.vps,
		sps:         t.sps,
		pps:         t.pps,
	}
}

// VPS returns the track VPS.
func (t *TrackH265) VPS() []byte {
	t.mutex.RLock()
	defer t.mutex.RUnlock()
	return t.vps
}

// SPS returns the track SPS.
func (t *TrackH265) SPS() []byte {
	t.mutex.RLock()
	defer t.mutex.RUnlock()
	return t.sps
}

// PPS returns the track PPS.
func (t *TrackH265) PPS() []byte {
	t.mutex.RLock()
	defer t.mutex.RUnlock()
	return t.pps
}

// SetVPS sets the track VPS.
func (t *TrackH265) SetVPS(v []byte) {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	t.vps = v
}

// SetSPS sets the track SPS.
func (t *TrackH265) SetSPS(v []byte) {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	t.sps = v
}

// SetPPS sets the track PPS.
func (t *TrackH265) SetPPS(v []byte) {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	t.pps = v
}

// MediaDescription returns the track media description in SDP format.
func (t *TrackH265) MediaDescription() *psdp.MediaDescription {
	t.mutex.RLock()
	defer t.mutex.RUnlock()

	typ := strconv.FormatInt(int64(t.payloadType), 10)

	fmtp := typ

	var tmp []string
	if t.vps != nil {
		tmp = append(tmp, "sprop-vps="+base64.StdEncoding.EncodeToString(t.vps))
	}
	if t.sps != nil {
		tmp = append(tmp, "sprop-sps="+base64.StdEncoding.EncodeToString(t.sps))
	}
	if t.pps != nil {
		tmp = append(tmp, "sprop-pps="+base64.StdEncoding.EncodeToString(t.pps))
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
