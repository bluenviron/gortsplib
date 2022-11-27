package gortsplib

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/pion/rtp"
)

func findClockRate(payloadType uint8, rtpMap string) (int, error) {
	// get clock rate from payload type
	// https://en.wikipedia.org/wiki/RTP_payload_formats
	switch payloadType {
	case 0, 1, 2, 3, 4, 5, 7, 8, 9, 12, 13, 15, 18:
		return 8000, nil

	case 6:
		return 16000, nil

	case 10, 11:
		return 44100, nil

	case 14, 25, 26, 28, 31, 32, 33, 34:
		return 90000, nil

	case 16:
		return 11025, nil

	case 17:
		return 22050, nil
	}

	// get clock rate from rtpmap
	// https://tools.ietf.org/html/rfc4566
	// a=rtpmap:<payload type> <encoding name>/<clock rate> [/<encoding parameters>]
	if rtpMap == "" {
		return 0, fmt.Errorf("attribute 'rtpmap' not found")
	}

	tmp := strings.Split(rtpMap, "/")
	if len(tmp) != 2 && len(tmp) != 3 {
		return 0, fmt.Errorf("invalid rtpmap (%v)", rtpMap)
	}

	v, err := strconv.ParseInt(tmp[1], 10, 64)
	if err != nil {
		return 0, err
	}

	return int(v), nil
}

// TrackGeneric is a generic track.
type TrackGeneric struct {
	PayloadType uint8
	RTPMap      string
	FMTP        string

	clockRate int
}

// Init initializes a TrackGeneric
func (t *TrackGeneric) Init() error {
	t.clockRate, _ = findClockRate(t.PayloadType, t.RTPMap)
	return nil
}

// String returns a description of the track.
func (t *TrackGeneric) String() string {
	return "Generic"
}

// ClockRate returns the clock rate.
func (t *TrackGeneric) ClockRate() int {
	return t.clockRate
}

// GetPayloadType returns the payload type.
func (t *TrackGeneric) GetPayloadType() uint8 {
	return t.PayloadType
}

func (t *TrackGeneric) unmarshal(payloadType uint8, clock string, codec string, rtpmap string, fmtp string) error {
	t.PayloadType = payloadType
	t.RTPMap = rtpmap
	t.FMTP = fmtp

	return t.Init()
}

func (t *TrackGeneric) marshal() (string, string) {
	return t.RTPMap, t.FMTP
}

func (t *TrackGeneric) clone() Track {
	return &TrackGeneric{
		PayloadType: t.PayloadType,
		RTPMap:      t.RTPMap,
		FMTP:        t.FMTP,
		clockRate:   t.clockRate,
	}
}

func (t *TrackGeneric) ptsEqualsDTS(*rtp.Packet) bool {
	return true
}
