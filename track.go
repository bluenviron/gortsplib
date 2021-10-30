package gortsplib

import (
	"fmt"
	"strconv"
	"strings"

	psdp "github.com/pion/sdp/v3"

	"github.com/aler9/gortsplib/pkg/base"
	"github.com/aler9/gortsplib/pkg/sdp"
)

// Track is a RTSP track.
type Track struct {
	// attributes in SDP format
	Media *psdp.MediaDescription
}

func (t *Track) hasControlAttribute() bool {
	for _, attr := range t.Media.Attributes {
		if attr.Key == "control" {
			return true
		}
	}
	return false
}

// URL returns the track URL.
func (t *Track) URL(baseURL *base.URL) (*base.URL, error) {
	if baseURL == nil {
		return nil, fmt.Errorf("empty base URL")
	}

	controlAttr := func() string {
		for _, attr := range t.Media.Attributes {
			if attr.Key == "control" {
				return attr.Value
			}
		}
		return ""
	}()

	// no control attribute, use base URL
	if controlAttr == "" {
		return baseURL, nil
	}

	// control attribute contains an absolute path
	if strings.HasPrefix(controlAttr, "rtsp://") {
		ur, err := base.ParseURL(controlAttr)
		if err != nil {
			return nil, err
		}

		// copy host and credentials
		ur.Host = baseURL.Host
		ur.User = baseURL.User
		return ur, nil
	}

	// control attribute contains a relative control attribute
	// insert the control attribute at the end of the URL
	// if there's a query, insert it after the query
	// otherwise insert it after the path
	strURL := baseURL.String()
	if controlAttr[0] != '?' && !strings.HasSuffix(strURL, "/") {
		strURL += "/"
	}
	ur, _ := base.ParseURL(strURL + controlAttr)
	return ur, nil
}

// ClockRate returns the clock rate of the track.
func (t *Track) ClockRate() (int, error) {
	if len(t.Media.MediaName.Formats) < 1 {
		return 0, fmt.Errorf("no formats provided")
	}

	// get clock rate from payload type
	switch t.Media.MediaName.Formats[0] {
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
	for _, a := range t.Media.Attributes {
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

// Tracks is a list of tracks.
type Tracks []*Track

// ReadTracks decodes tracks from SDP.
func ReadTracks(byts []byte) (Tracks, error) {
	desc := sdp.SessionDescription{}
	err := desc.Unmarshal(byts)
	if err != nil {
		return nil, err
	}

	tracks := make(Tracks, len(desc.MediaDescriptions))

	for i, media := range desc.MediaDescriptions {
		tracks[i] = &Track{
			Media: media,
		}
	}

	// since ReadTracks is used to handle ANNOUNCE and SETUP requests,
	// all tracks must have a valid clock rate.
	for i, track := range tracks {
		_, err := track.ClockRate()
		if err != nil {
			return nil, fmt.Errorf("unable to get clock rate of track %d: %s", i, err)
		}
	}

	return tracks, nil
}

func cloneAndClearTracks(ts Tracks) Tracks {
	ret := make(Tracks, len(ts))

	for i, track := range ts {
		md := &psdp.MediaDescription{
			MediaName: psdp.MediaName{
				Media:   track.Media.MediaName.Media,
				Protos:  []string{"RTP", "AVP"}, // override protocol
				Formats: track.Media.MediaName.Formats,
			},
			Bandwidth: track.Media.Bandwidth,
			Attributes: func() []psdp.Attribute {
				var ret []psdp.Attribute

				for _, attr := range track.Media.Attributes {
					if attr.Key == "rtpmap" || attr.Key == "fmtp" {
						ret = append(ret, attr)
					}
				}

				ret = append(ret, psdp.Attribute{
					Key:   "control",
					Value: "trackID=" + strconv.FormatInt(int64(i), 10),
				})

				return ret
			}(),
		}

		ret[i] = &Track{
			Media: md,
		}
	}

	return ret
}

// Write encodes tracks into SDP.
func (ts Tracks) Write() []byte {
	sout := &sdp.SessionDescription{
		SessionName: psdp.SessionName("Stream"),
		Origin: psdp.Origin{
			Username:       "-",
			NetworkType:    "IN",
			AddressType:    "IP4",
			UnicastAddress: "127.0.0.1",
		},
		// required by Darwin Streaming Server
		ConnectionInformation: &psdp.ConnectionInformation{
			NetworkType: "IN",
			AddressType: "IP4",
			Address:     &psdp.Address{Address: "0.0.0.0"},
		},
		TimeDescriptions: []psdp.TimeDescription{
			{Timing: psdp.Timing{0, 0}}, //nolint:govet
		},
	}

	for _, track := range ts {
		sout.MediaDescriptions = append(sout.MediaDescriptions, track.Media)
	}

	byts, _ := sout.Marshal()
	return byts
}
