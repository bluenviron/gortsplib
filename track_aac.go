package gortsplib

import (
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	psdp "github.com/pion/sdp/v3"

	"github.com/aler9/gortsplib/pkg/aac"
)

// TrackAAC is an AAC track.
type TrackAAC struct {
	trackBase
	payloadType       uint8
	typ               int
	sampleRate        int
	channelCount      int
	aotSpecificConfig []byte
	mpegConf          []byte
	sizeLength        int
	indexLength       int
	indexDeltaLength  int
}

// NewTrackAAC allocates a TrackAAC.
func NewTrackAAC(payloadType uint8,
	typ int,
	sampleRate int,
	channelCount int,
	aotSpecificConfig []byte,
	sizeLength int,
	indexLength int,
	indexDeltaLength int,
) (*TrackAAC, error) {
	mpegConf, err := aac.MPEG4AudioConfig{
		Type:              aac.MPEG4AudioType(typ),
		SampleRate:        sampleRate,
		ChannelCount:      channelCount,
		AOTSpecificConfig: aotSpecificConfig,
	}.Encode()
	if err != nil {
		return nil, fmt.Errorf("invalid configuration: %s", err)
	}

	return &TrackAAC{
		payloadType:       payloadType,
		typ:               typ,
		sampleRate:        sampleRate,
		channelCount:      channelCount,
		aotSpecificConfig: aotSpecificConfig,
		mpegConf:          mpegConf,
		sizeLength:        sizeLength,
		indexLength:       indexLength,
		indexDeltaLength:  indexDeltaLength,
	}, nil
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

	track := &TrackAAC{
		trackBase: trackBase{
			control: control,
		},
		payloadType: payloadType,
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

			var mpegConf aac.MPEG4AudioConfig
			err = mpegConf.Decode(enc)
			if err != nil {
				return nil, fmt.Errorf("invalid AAC config (%v)", tmp[1])
			}

			// re-encode the conf to normalize it
			enc, _ = mpegConf.Encode()

			track.typ = int(mpegConf.Type)
			track.sampleRate = mpegConf.SampleRate
			track.channelCount = mpegConf.ChannelCount
			track.aotSpecificConfig = mpegConf.AOTSpecificConfig
			track.mpegConf = enc

		case "sizelength":
			val, err := strconv.ParseUint(tmp[1], 10, 64)
			if err != nil {
				return nil, fmt.Errorf("invalid AAC sizeLength (%v)", tmp[1])
			}
			track.sizeLength = int(val)

		case "indexlength":
			val, err := strconv.ParseUint(tmp[1], 10, 64)
			if err != nil {
				return nil, fmt.Errorf("invalid AAC indexLength (%v)", tmp[1])
			}
			track.indexLength = int(val)

		case "indexdeltalength":
			val, err := strconv.ParseUint(tmp[1], 10, 64)
			if err != nil {
				return nil, fmt.Errorf("invalid AAC indexDeltaLength (%v)", tmp[1])
			}
			track.indexDeltaLength = int(val)
		}
	}

	if len(track.mpegConf) == 0 {
		return nil, fmt.Errorf("config is missing (%v)", v)
	}

	if track.sizeLength == 0 {
		return nil, fmt.Errorf("sizelength is missing (%v)", v)
	}

	if track.indexLength == 0 && track.sizeLength == 13 {
		track.indexLength = 3
	}

	if track.indexDeltaLength == 0 && track.sizeLength == 13 {
		track.indexDeltaLength = 3
	}

	return track, nil
}

// ClockRate returns the track clock rate.
func (t *TrackAAC) ClockRate() int {
	return t.sampleRate
}

// Type returns the track MPEG4-audio type.
func (t *TrackAAC) Type() int {
	return t.typ
}

// ChannelCount returns the track channel count.
func (t *TrackAAC) ChannelCount() int {
	return t.channelCount
}

// AOTSpecificConfig returns the track AOT specific config.
func (t *TrackAAC) AOTSpecificConfig() []byte {
	return t.aotSpecificConfig
}

// SizeLength returns the track sizeLength.
func (t *TrackAAC) SizeLength() int {
	return t.sizeLength
}

// IndexLength returns the track indexLength.
func (t *TrackAAC) IndexLength() int {
	return t.indexLength
}

// IndexDeltaLength returns the track indexDeltaLength.
func (t *TrackAAC) IndexDeltaLength() int {
	return t.indexDeltaLength
}

func (t *TrackAAC) clone() Track {
	return &TrackAAC{
		trackBase:         t.trackBase,
		payloadType:       t.payloadType,
		typ:               t.typ,
		sampleRate:        t.sampleRate,
		channelCount:      t.channelCount,
		aotSpecificConfig: t.aotSpecificConfig,
		mpegConf:          t.mpegConf,
		sizeLength:        t.sizeLength,
		indexLength:       t.indexLength,
		indexDeltaLength:  t.indexDeltaLength,
	}
}

// MediaDescription returns the track media description in SDP format.
func (t *TrackAAC) MediaDescription() *psdp.MediaDescription {
	typ := strconv.FormatInt(int64(t.payloadType), 10)

	return &psdp.MediaDescription{
		MediaName: psdp.MediaName{
			Media:   "audio",
			Protos:  []string{"RTP", "AVP"},
			Formats: []string{typ},
		},
		Attributes: []psdp.Attribute{
			{
				Key: "rtpmap",
				Value: typ + " mpeg4-generic/" + strconv.FormatInt(int64(t.sampleRate), 10) +
					"/" + strconv.FormatInt(int64(t.channelCount), 10),
			},
			{
				Key: "fmtp",
				Value: typ + " profile-level-id=1; " +
					"mode=AAC-hbr; " +
					func() string {
						if t.sizeLength > 0 {
							return fmt.Sprintf("sizelength=%d; ", t.sizeLength)
						}
						return ""
					}() +
					func() string {
						if t.indexLength > 0 {
							return fmt.Sprintf("indexlength=%d; ", t.indexLength)
						}
						return ""
					}() +
					func() string {
						if t.indexDeltaLength > 0 {
							return fmt.Sprintf("indexdeltalength=%d; ", t.indexDeltaLength)
						}
						return ""
					}() +
					"config=" + hex.EncodeToString(t.mpegConf),
			},
			{
				Key:   "control",
				Value: t.control,
			},
		},
	}
}
