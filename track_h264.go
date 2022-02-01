package gortsplib

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	psdp "github.com/pion/sdp/v3"

	"github.com/aler9/gortsplib/pkg/base"
)

func trackH264GetSPSPPS(md *psdp.MediaDescription) ([]byte, []byte, error) {
	v, ok := md.Attribute("fmtp")
	if !ok {
		return nil, nil, fmt.Errorf("fmtp attribute is missing")
	}

	tmp := strings.SplitN(v, " ", 2)
	if len(tmp) != 2 {
		return nil, nil, fmt.Errorf("invalid fmtp attribute (%v)", v)
	}

	for _, kv := range strings.Split(tmp[1], ";") {
		kv = strings.Trim(kv, " ")

		if len(kv) == 0 {
			continue
		}

		tmp := strings.SplitN(kv, "=", 2)
		if len(tmp) != 2 {
			return nil, nil, fmt.Errorf("invalid fmtp attribute (%v)", v)
		}

		if tmp[0] == "sprop-parameter-sets" {
			tmp := strings.Split(tmp[1], ",")
			if len(tmp) < 2 {
				return nil, nil, fmt.Errorf("invalid sprop-parameter-sets (%v)", v)
			}

			sps, err := base64.StdEncoding.DecodeString(tmp[0])
			if err != nil {
				return nil, nil, fmt.Errorf("invalid sprop-parameter-sets (%v)", v)
			}

			pps, err := base64.StdEncoding.DecodeString(tmp[1])
			if err != nil {
				return nil, nil, fmt.Errorf("invalid sprop-parameter-sets (%v)", v)
			}

			return sps, pps, nil
		}
	}

	return nil, nil, fmt.Errorf("sprop-parameter-sets is missing (%v)", v)
}

// TrackH264 is a H264 track.
type TrackH264 struct {
	control     string
	payloadType uint8
	sps         []byte
	pps         []byte
	extradata   []byte
}

// NewTrackH264 allocates a TrackH264.
func NewTrackH264(payloadType uint8, sps []byte, pps []byte, extradata []byte) (*TrackH264, error) {
	return &TrackH264{
		payloadType: payloadType,
		sps:         sps,
		pps:         pps,
		extradata:   extradata,
	}, nil
}

func newTrackH264FromMediaDescription(payloadType uint8,
	md *psdp.MediaDescription) (*TrackH264, error) {
	control := trackFindControl(md)

	t := &TrackH264{
		control:     control,
		payloadType: payloadType,
	}

	sps, pps, err := trackH264GetSPSPPS(md)
	if err == nil {
		t.sps = sps
		t.pps = pps
	}

	return t, nil
}

// ClockRate returns the track clock rate.
func (t *TrackH264) ClockRate() int {
	return 90000
}

func (t *TrackH264) clone() Track {
	return &TrackH264{
		control:     t.control,
		payloadType: t.payloadType,
		sps:         t.sps,
		pps:         t.pps,
		extradata:   t.extradata,
	}
}

func (t *TrackH264) getControl() string {
	return t.control
}

func (t *TrackH264) setControl(c string) {
	t.control = c
}

func (t *TrackH264) url(contentBase *base.URL) (*base.URL, error) {
	return trackURL(t, contentBase)
}

// SPS returns the track SPS.
func (t *TrackH264) SPS() []byte {
	return t.sps
}

// PPS returns the track PPS.
func (t *TrackH264) PPS() []byte {
	return t.pps
}

// SetSPS sets the track SPS.
func (t *TrackH264) SetSPS(v []byte) {
	t.sps = v
}

// SetPPS sets the track PPS.
func (t *TrackH264) SetPPS(v []byte) {
	t.pps = v
}

func (t *TrackH264) mediaDescription() *psdp.MediaDescription {
	typ := strconv.FormatInt(int64(t.payloadType), 10)

	fmtp := typ + " packetization-mode=1"

	var tmp []string
	if t.sps != nil {
		tmp = append(tmp, base64.StdEncoding.EncodeToString(t.sps))
	}
	if t.pps != nil {
		tmp = append(tmp, base64.StdEncoding.EncodeToString(t.pps))
	}
	if t.extradata != nil {
		tmp = append(tmp, base64.StdEncoding.EncodeToString(t.extradata))
	}
	fmtp += "; sprop-parameter-sets=" + strings.Join(tmp, ",")

	if len(t.sps) >= 4 {
		fmtp += "; profile-level-id=" + strings.ToUpper(hex.EncodeToString(t.sps[1:4]))
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
				Value: typ + " H264/90000",
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
