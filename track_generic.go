package gortsplib

import (
	"fmt"
	"strconv"
	"strings"

	psdp "github.com/pion/sdp/v3"

	"github.com/aler9/gortsplib/pkg/base"
)

func trackGenericGetClockRate(md *psdp.MediaDescription) (int, error) {
	if len(md.MediaName.Formats) < 1 {
		return 0, fmt.Errorf("no formats provided")
	}

	// get clock rate from payload type
	// https://en.wikipedia.org/wiki/RTP_payload_formats
	switch md.MediaName.Formats[0] {
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
	for _, a := range md.Attributes {
		if a.Key == "rtpmap" {
			tmp := strings.Split(a.Value, " ")
			if len(tmp) < 2 {
				return 0, fmt.Errorf("invalid rtpmap (%v)", a.Value)
			}

			tmp = strings.Split(tmp[1], "/")
			if len(tmp) != 2 && len(tmp) != 3 {
				return 0, fmt.Errorf("invalid rtpmap (%v)", a.Value)
			}

			v, err := strconv.ParseInt(tmp[1], 10, 64)
			if err != nil {
				return 0, err
			}
			return int(v), nil
		}
	}

	return 0, fmt.Errorf("attribute 'rtpmap' not found")
}

// TrackGeneric is a generic track.
type TrackGeneric struct {
	control   string
	clockRate int
	media     string
	formats   []string
	rtpmap    string
	fmtp      string
}

func newTrackGenericFromMediaDescription(md *psdp.MediaDescription) (*TrackGeneric, error) {
	control := trackFindControl(md)

	clockRate, err := trackGenericGetClockRate(md)
	if err != nil {
		return nil, fmt.Errorf("unable to get clock rate: %s", err)
	}

	rtpmap := func() string {
		for _, attr := range md.Attributes {
			if attr.Key == "rtpmap" {
				return attr.Value
			}
		}
		return ""
	}()

	fmtp := func() string {
		for _, attr := range md.Attributes {
			if attr.Key == "fmtp" {
				return attr.Value
			}
		}
		return ""
	}()

	return &TrackGeneric{
		control:   control,
		clockRate: clockRate,
		media:     md.MediaName.Media,
		formats:   md.MediaName.Formats,
		rtpmap:    rtpmap,
		fmtp:      fmtp,
	}, nil
}

// ClockRate returns the track clock rate.
func (t *TrackGeneric) ClockRate() int {
	return t.clockRate
}

func (t *TrackGeneric) clone() Track {
	return &TrackGeneric{
		control:   t.control,
		clockRate: t.clockRate,
		media:     t.media,
		formats:   t.formats,
		rtpmap:    t.rtpmap,
		fmtp:      t.fmtp,
	}
}

func (t *TrackGeneric) getControl() string {
	return t.control
}

func (t *TrackGeneric) setControl(c string) {
	t.control = c
}

func (t *TrackGeneric) url(contentBase *base.URL) (*base.URL, error) {
	return trackURL(t, contentBase)
}

func (t *TrackGeneric) mediaDescription() *psdp.MediaDescription {
	return &psdp.MediaDescription{
		MediaName: psdp.MediaName{
			Media:   t.media,
			Protos:  []string{"RTP", "AVP"},
			Formats: t.formats,
		},
		Attributes: func() []psdp.Attribute {
			var ret []psdp.Attribute

			if t.rtpmap != "" {
				ret = append(ret, psdp.Attribute{
					Key:   "rtpmap",
					Value: t.rtpmap,
				})
			}

			if t.fmtp != "" {
				ret = append(ret, psdp.Attribute{
					Key:   "fmtp",
					Value: t.fmtp,
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
