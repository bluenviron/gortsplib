package gortsplib

import (
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	psdp "github.com/pion/sdp/v3"

	"github.com/aler9/gortsplib/pkg/mpeg4audio"
)

// TrackAAC is an AAC track.
type TrackAAC struct {
	PayloadType      uint8
	Config           *mpeg4audio.Config
	SizeLength       int
	IndexLength      int
	IndexDeltaLength int

	trackBase
}

func newTrackAACFromMediaDescription(
	control string,
	payloadType uint8,
	md *psdp.MediaDescription,
) (*TrackAAC, error) {
	v, ok := md.Attribute("fmtp")
	if !ok {
		return nil, fmt.Errorf("fmtp attribute is missing")
	}

	tmp := strings.SplitN(v, " ", 2)
	if len(tmp) != 2 {
		return nil, fmt.Errorf("invalid fmtp (%v)", v)
	}

	t := &TrackAAC{
		PayloadType: payloadType,
		trackBase: trackBase{
			control: control,
		},
	}

	for _, kv := range strings.Split(tmp[1], ";") {
		kv = strings.Trim(kv, " ")

		if len(kv) == 0 {
			continue
		}

		tmp := strings.SplitN(kv, "=", 2)
		if len(tmp) != 2 {
			return nil, fmt.Errorf("invalid fmtp (%v)", v)
		}

		switch strings.ToLower(tmp[0]) {
		case "config":
			enc, err := hex.DecodeString(tmp[1])
			if err != nil {
				return nil, fmt.Errorf("invalid AAC config (%v)", tmp[1])
			}

			t.Config = &mpeg4audio.Config{}
			err = t.Config.Unmarshal(enc)
			if err != nil {
				return nil, fmt.Errorf("invalid AAC config (%v)", tmp[1])
			}

		case "sizelength":
			val, err := strconv.ParseUint(tmp[1], 10, 64)
			if err != nil {
				return nil, fmt.Errorf("invalid AAC SizeLength (%v)", tmp[1])
			}
			t.SizeLength = int(val)

		case "indexlength":
			val, err := strconv.ParseUint(tmp[1], 10, 64)
			if err != nil {
				return nil, fmt.Errorf("invalid AAC IndexLength (%v)", tmp[1])
			}
			t.IndexLength = int(val)

		case "indexdeltalength":
			val, err := strconv.ParseUint(tmp[1], 10, 64)
			if err != nil {
				return nil, fmt.Errorf("invalid AAC IndexDeltaLength (%v)", tmp[1])
			}
			t.IndexDeltaLength = int(val)
		}
	}

	if t.Config == nil {
		return nil, fmt.Errorf("config is missing (%v)", v)
	}

	if t.SizeLength == 0 {
		return nil, fmt.Errorf("sizelength is missing (%v)", v)
	}

	return t, nil
}

// ClockRate returns the track clock rate.
func (t *TrackAAC) ClockRate() int {
	return t.Config.SampleRate
}

func (t *TrackAAC) clone() Track {
	return &TrackAAC{
		PayloadType:      t.PayloadType,
		Config:           t.Config,
		SizeLength:       t.SizeLength,
		IndexLength:      t.IndexLength,
		IndexDeltaLength: t.IndexDeltaLength,
		trackBase:        t.trackBase,
	}
}

// MediaDescription returns the track media description in SDP format.
func (t *TrackAAC) MediaDescription() *psdp.MediaDescription {
	enc, err := t.Config.Marshal()
	if err != nil {
		return nil
	}

	typ := strconv.FormatInt(int64(t.PayloadType), 10)

	return &psdp.MediaDescription{
		MediaName: psdp.MediaName{
			Media:   "audio",
			Protos:  []string{"RTP", "AVP"},
			Formats: []string{typ},
		},
		Attributes: []psdp.Attribute{
			{
				Key: "rtpmap",
				Value: typ + " mpeg4-generic/" + strconv.FormatInt(int64(t.Config.SampleRate), 10) +
					"/" + strconv.FormatInt(int64(t.Config.ChannelCount), 10),
			},
			{
				Key: "fmtp",
				Value: typ + " profile-level-id=1; " +
					"mode=AAC-hbr; " +
					func() string {
						if t.SizeLength > 0 {
							return fmt.Sprintf("sizelength=%d; ", t.SizeLength)
						}
						return ""
					}() +
					func() string {
						if t.IndexLength > 0 {
							return fmt.Sprintf("indexlength=%d; ", t.IndexLength)
						}
						return ""
					}() +
					func() string {
						if t.IndexDeltaLength > 0 {
							return fmt.Sprintf("indexdeltalength=%d; ", t.IndexDeltaLength)
						}
						return ""
					}() +
					"config=" + hex.EncodeToString(enc),
			},
			{
				Key:   "control",
				Value: t.control,
			},
		},
	}
}
