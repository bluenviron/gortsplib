package gortsplib

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	"github.com/notedit/rtmp/codec/aac"
	psdp "github.com/pion/sdp/v3"

	"github.com/aler9/gortsplib/pkg/base"
	"github.com/aler9/gortsplib/pkg/sdp"
)

// Track is a track available in a certain URL.
type Track struct {
	// base url
	BaseURL *base.URL

	// id
	ID int

	// codec and info in SDP format
	Media *psdp.MediaDescription
}

// NewTrackH264 initializes an H264 track.
func NewTrackH264(payloadType uint8, sps []byte, pps []byte) (*Track, error) {
	spropParameterSets := base64.StdEncoding.EncodeToString(sps) +
		"," + base64.StdEncoding.EncodeToString(pps)
	profileLevelID := strings.ToUpper(hex.EncodeToString(sps[1:4]))

	typ := strconv.FormatInt(int64(payloadType), 10)

	return &Track{
		Media: &psdp.MediaDescription{
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
					Key: "fmtp",
					Value: typ + " packetization-mode=1; " +
						"sprop-parameter-sets=" + spropParameterSets + "; " +
						"profile-level-id=" + profileLevelID,
				},
			},
		},
	}, nil
}

// NewTrackAAC initializes an AAC track.
func NewTrackAAC(payloadType uint8, config []byte) (*Track, error) {
	codec, err := aac.FromMPEG4AudioConfigBytes(config)
	if err != nil {
		return nil, err
	}

	// https://github.com/notedit/rtmp/blob/6e314ac5b29611431f8fb5468596b05815743c10/codec/aac/aac.go#L106
	channelCount, err := func() (int, error) {
		if codec.Config.ChannelConfig >= 1 && codec.Config.ChannelConfig <= 6 {
			return int(codec.Config.ChannelConfig), nil
		}

		if codec.Config.ChannelConfig == 8 {
			return 7, nil
		}

		return 0, fmt.Errorf("unsupported channel config: %v", codec.Config.ChannelConfig)
	}()
	if err != nil {
		return nil, err
	}

	typ := strconv.FormatInt(int64(payloadType), 10)

	return &Track{
		Media: &psdp.MediaDescription{
			MediaName: psdp.MediaName{
				Media:   "audio",
				Protos:  []string{"RTP", "AVP"},
				Formats: []string{typ},
			},
			Attributes: []psdp.Attribute{
				{
					Key: "rtpmap",
					Value: typ + " MPEG4-GENERIC/" + strconv.FormatInt(int64(codec.Config.SampleRate), 10) +
						"/" + strconv.FormatInt(int64(channelCount), 10),
				},
				{
					Key: "fmtp",
					Value: typ + " profile-level-id=1; " +
						"mode=AAC-hbr; " +
						"sizelength=13; " +
						"indexlength=3; " +
						"indexdeltalength=3; " +
						"config=" + hex.EncodeToString(config),
				},
			},
		},
	}, nil
}

// ClockRate returns the clock rate of the track.
func (t *Track) ClockRate() (int, error) {
	if len(t.Media.MediaName.Formats) != 1 {
		return 0, fmt.Errorf("invalid format (%v)", t.Media.MediaName.Formats)
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

// URL returns the track url.
func (t *Track) URL() (*base.URL, error) {
	if t.BaseURL == nil {
		return nil, fmt.Errorf("empty base url")
	}

	control := func() string {
		for _, attr := range t.Media.Attributes {
			if attr.Key == "control" {
				return attr.Value
			}
		}
		return ""
	}()

	// no control attribute, use base URL
	if control == "" {
		return t.BaseURL, nil
	}

	// control attribute contains an absolute path
	if strings.HasPrefix(control, "rtsp://") {
		ur, err := base.ParseURL(control)
		if err != nil {
			return nil, err
		}

		// copy host and credentials
		ur.Host = t.BaseURL.Host
		ur.User = t.BaseURL.User
		return ur, nil
	}

	// control attribute contains a relative control attribute
	ur := t.BaseURL.Clone()
	ur.AddControlAttribute(control)
	return ur, nil
}

// Tracks is a list of tracks.
type Tracks []*Track

// ReadTracks reads tracks from an encoded SDP.
func ReadTracks(byts []byte) (Tracks, error) {
	desc := sdp.SessionDescription{}
	err := desc.Unmarshal(byts)
	if err != nil {
		return nil, err
	}

	tracks := make(Tracks, len(desc.MediaDescriptions))

	for i, media := range desc.MediaDescriptions {
		tracks[i] = &Track{
			ID:    i,
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

// Write writes tracks in SDP format.
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

	for i, track := range ts {
		mout := &psdp.MediaDescription{
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

				// control attribute is the path that is appended
				// to the stream path in SETUP
				ret = append(ret, psdp.Attribute{
					Key:   "control",
					Value: "trackID=" + strconv.FormatInt(int64(i), 10),
				})

				return ret
			}(),
		}
		sout.MediaDescriptions = append(sout.MediaDescriptions, mout)
	}

	byts, _ := sout.Marshal()
	return byts
}
