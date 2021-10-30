package gortsplib

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	psdp "github.com/pion/sdp/v3"
)

// TrackConfigH264 is the configuration of an H264 track.
type TrackConfigH264 struct {
	SPS []byte
	PPS []byte
}

// NewTrackH264 initializes an H264 track.
func NewTrackH264(payloadType uint8, conf *TrackConfigH264) (*Track, error) {
	if len(conf.SPS) < 4 {
		return nil, fmt.Errorf("invalid SPS")
	}

	spropParameterSets := base64.StdEncoding.EncodeToString(conf.SPS) +
		"," + base64.StdEncoding.EncodeToString(conf.PPS)
	profileLevelID := strings.ToUpper(hex.EncodeToString(conf.SPS[1:4]))

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

// IsH264 checks whether the track is an H264 track.
func (t *Track) IsH264() bool {
	if t.Media.MediaName.Media != "video" {
		return false
	}

	v, ok := t.Media.Attribute("rtpmap")
	if !ok {
		return false
	}

	v = strings.TrimSpace(v)
	vals := strings.Split(v, " ")
	if len(vals) != 2 {
		return false
	}

	return vals[1] == "H264/90000"
}

// ExtractConfigH264 extracts the configuration of an H264 track.
func (t *Track) ExtractConfigH264() (*TrackConfigH264, error) {
	v, ok := t.Media.Attribute("fmtp")
	if !ok {
		return nil, fmt.Errorf("fmtp attribute is missing")
	}

	tmp := strings.SplitN(v, " ", 2)
	if len(tmp) != 2 {
		return nil, fmt.Errorf("invalid fmtp attribute (%v)", v)
	}

	for _, kv := range strings.Split(tmp[1], ";") {
		kv = strings.Trim(kv, " ")

		if len(kv) == 0 {
			continue
		}

		tmp := strings.SplitN(kv, "=", 2)
		if len(tmp) != 2 {
			return nil, fmt.Errorf("invalid fmtp attribute (%v)", v)
		}

		if tmp[0] == "sprop-parameter-sets" {
			tmp := strings.SplitN(tmp[1], ",", 2)
			if len(tmp) != 2 {
				return nil, fmt.Errorf("invalid sprop-parameter-sets (%v)", v)
			}

			sps, err := base64.StdEncoding.DecodeString(tmp[0])
			if err != nil {
				return nil, fmt.Errorf("invalid sprop-parameter-sets (%v)", v)
			}

			pps, err := base64.StdEncoding.DecodeString(tmp[1])
			if err != nil {
				return nil, fmt.Errorf("invalid sprop-parameter-sets (%v)", v)
			}

			conf := &TrackConfigH264{
				SPS: sps,
				PPS: pps,
			}

			return conf, nil
		}
	}

	return nil, fmt.Errorf("sprop-parameter-sets is missing (%v)", v)
}
