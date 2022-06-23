package gortsplib

import (
	"fmt"
	"strconv"
	"strings"

	psdp "github.com/pion/sdp/v3"
)

func trackGenericGetClockRate(formats []string, rtpmap string) (int, error) {
	if len(formats) < 1 {
		return 0, fmt.Errorf("no formats provided")
	}

	// get clock rate from payload type
	// https://en.wikipedia.org/wiki/RTP_payload_formats
	switch formats[0] {
	case "0", "1", "2", "3", "4", "5", "7", "8", "9", "12", "13", "15", "18":
		return 8000, nil

	case "6":
		return 16000, nil

	case "10", "11":
		return 44100, nil

	case "14", "25", "26", "28", "31", "32", "33", "34":
		return 90000, nil

	case "16":
		return 11025, nil

	case "17":
		return 22050, nil
	}

	// get clock rate from rtpmap
	// https://tools.ietf.org/html/rfc4566
	// a=rtpmap:<payload type> <encoding name>/<clock rate> [/<encoding parameters>]
	if rtpmap == "" {
		return 0, fmt.Errorf("attribute 'rtpmap' not found")
	}

	tmp := strings.Split(rtpmap, " ")
	if len(tmp) < 2 {
		return 0, fmt.Errorf("invalid rtpmap (%v)", rtpmap)
	}

	tmp = strings.Split(tmp[1], "/")
	if len(tmp) != 2 && len(tmp) != 3 {
		return 0, fmt.Errorf("invalid rtpmap (%v)", rtpmap)
	}

	v, err := strconv.ParseInt(tmp[1], 10, 64)
	if err != nil {
		return 0, err
	}
	return int(v), nil
}

// TrackGeneric is a generic track.
type TrackGeneric struct {
	Media   string
	Formats []string
	RTPMap  string
	FMTP    string

	trackBase
}

func newTrackGenericFromMediaDescription(
	control string,
	md *psdp.MediaDescription,
) (*TrackGeneric, error) {
	rtpmap := func() string {
		for _, attr := range md.Attributes {
			if attr.Key == "rtpmap" {
				return attr.Value
			}
		}
		return ""
	}()

	_, err := trackGenericGetClockRate(md.MediaName.Formats, rtpmap)
	if err != nil {
		return nil, fmt.Errorf("unable to get clock rate: %s", err)
	}

	fmtp := func() string {
		for _, attr := range md.Attributes {
			if attr.Key == "fmtp" {
				return attr.Value
			}
		}
		return ""
	}()

	return &TrackGeneric{
		Media:   md.MediaName.Media,
		Formats: md.MediaName.Formats,
		RTPMap:  rtpmap,
		FMTP:    fmtp,
		trackBase: trackBase{
			control: control,
		},
	}, nil
}

// ClockRate returns the track clock rate.
func (t *TrackGeneric) ClockRate() int {
	clockRate, _ := trackGenericGetClockRate(t.Formats, t.RTPMap)
	return clockRate
}

func (t *TrackGeneric) clone() Track {
	return &TrackGeneric{
		Media:     t.Media,
		Formats:   t.Formats,
		RTPMap:    t.RTPMap,
		FMTP:      t.FMTP,
		trackBase: t.trackBase,
	}
}

// MediaDescription returns the track media description in SDP format.
func (t *TrackGeneric) MediaDescription() *psdp.MediaDescription {
	return &psdp.MediaDescription{
		MediaName: psdp.MediaName{
			Media:   t.Media,
			Protos:  []string{"RTP", "AVP"},
			Formats: t.Formats,
		},
		Attributes: func() []psdp.Attribute {
			var ret []psdp.Attribute

			if t.RTPMap != "" {
				ret = append(ret, psdp.Attribute{
					Key:   "rtpmap",
					Value: t.RTPMap,
				})
			}

			if t.FMTP != "" {
				ret = append(ret, psdp.Attribute{
					Key:   "fmtp",
					Value: t.FMTP,
				})
			}

			ret = append(ret, psdp.Attribute{
				Key:   "control",
				Value: t.control,
			})

			return ret
		}(),
	}
}
