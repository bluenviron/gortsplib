package gortsplib

import (
	"fmt"
	"strconv"
	"strings"

	psdp "github.com/pion/sdp/v3"
)

func findClockRate(track *TrackGeneric) (int, error) {
	// RFC 4566
	// When a list of
	// payload type numbers is given, this implies that all of these
	// payload formats MAY be used in the session, but the first of these
	// formats SHOULD be used as the default format for the session
	payload := track.Payloads[0]

	// get clock rate from payload type
	// https://en.wikipedia.org/wiki/RTP_payload_formats
	switch payload.Type {
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
	if payload.RTPMap == "" {
		return 0, fmt.Errorf("attribute 'rtpmap' not found")
	}

	tmp := strings.Split(payload.RTPMap, "/")
	if len(tmp) != 2 && len(tmp) != 3 {
		return 0, fmt.Errorf("invalid rtpmap (%v)", payload.RTPMap)
	}

	v, err := strconv.ParseInt(tmp[1], 10, 64)
	if err != nil {
		return 0, err
	}

	return int(v), nil
}

// TrackGenericPayload is a payload of a TrackGeneric.
type TrackGenericPayload struct {
	Type   uint8
	RTPMap string
	FMTP   string
}

// TrackGeneric is a generic track.
type TrackGeneric struct {
	Media    string
	Payloads []TrackGenericPayload

	trackBase

	clockRate int
}

func newTrackGenericFromMediaDescription(
	control string,
	md *psdp.MediaDescription,
) (*TrackGeneric, error) {
	t := &TrackGeneric{
		Media: md.MediaName.Media,
		trackBase: trackBase{
			control: control,
		},
	}

	for _, format := range md.MediaName.Formats {
		tmp, err := strconv.ParseInt(format, 10, 8)
		if err != nil {
			return nil, err
		}
		payloadType := uint8(tmp)

		t.Payloads = append(t.Payloads, TrackGenericPayload{
			Type:   payloadType,
			RTPMap: getRtpmapAttribute(md.Attributes, payloadType),
			FMTP:   getFmtpAttribute(md.Attributes, payloadType),
		})
	}

	err := t.Init()
	if err != nil {
		return nil, err
	}

	return t, nil
}

// Init initializes a TrackGeneric
func (t *TrackGeneric) Init() error {
	t.clockRate, _ = findClockRate(t)
	return nil
}

// String returns the track codec.
func (t *TrackGeneric) String() string {
	return "Generic"
}

// ClockRate returns the track clock rate.
func (t *TrackGeneric) ClockRate() int {
	return t.clockRate
}

// MediaDescription returns the track media description in SDP format.
func (t *TrackGeneric) MediaDescription() *psdp.MediaDescription {
	formats := make([]string, len(t.Payloads))
	for i, pl := range t.Payloads {
		formats[i] = strconv.FormatInt(int64(pl.Type), 10)
	}

	var attributes []psdp.Attribute

	for _, pl := range t.Payloads {
		if pl.RTPMap != "" {
			attributes = append(attributes, psdp.Attribute{
				Key:   "rtpmap",
				Value: strconv.FormatInt(int64(pl.Type), 10) + " " + pl.RTPMap,
			})
		}
		if pl.FMTP != "" {
			attributes = append(attributes, psdp.Attribute{
				Key:   "fmtp",
				Value: strconv.FormatInt(int64(pl.Type), 10) + " " + pl.FMTP,
			})
		}
	}

	attributes = append(attributes, psdp.Attribute{
		Key:   "control",
		Value: t.control,
	})

	return &psdp.MediaDescription{
		MediaName: psdp.MediaName{
			Media:   t.Media,
			Protos:  []string{"RTP", "AVP"},
			Formats: formats,
		},
		Attributes: attributes,
	}
}

func (t *TrackGeneric) clone() Track {
	return &TrackGeneric{
		Media:     t.Media,
		Payloads:  append([]TrackGenericPayload(nil), t.Payloads...),
		trackBase: t.trackBase,
		clockRate: t.clockRate,
	}
}
