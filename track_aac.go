package gortsplib

import (
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	psdp "github.com/pion/sdp/v3"

	"github.com/aler9/gortsplib/pkg/aac"
	"github.com/aler9/gortsplib/pkg/base"
)

// TrackAAC is an AAC track.
type TrackAAC struct {
	control           string
	payloadType       uint8
	typ               int
	sampleRate        int
	channelCount      int
	aotSpecificConfig []byte
	mpegConf          []byte
}

// NewTrackAAC allocates a TrackAAC.
func NewTrackAAC(payloadType uint8, typ int, sampleRate int,
	channelCount int, aotSpecificConfig []byte) (*TrackAAC, error) {
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
	}, nil
}

func newTrackAACFromMediaDescription(payloadType uint8, md *psdp.MediaDescription) (*TrackAAC, error) {
	control := trackFindControl(md)

	v, ok := md.Attribute("fmtp")
	if !ok {
		return nil, fmt.Errorf("fmtp attribute is missing")
	}

	tmp := strings.SplitN(v, " ", 2)
	if len(tmp) != 2 {
		return nil, fmt.Errorf("invalid fmtp (%v)", v)
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

		if tmp[0] == "config" {
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
			enc, err = mpegConf.Encode()
			if err != nil {
				return nil, fmt.Errorf("invalid AAC config (%v)", tmp[1])
			}

			return &TrackAAC{
				control:           control,
				payloadType:       payloadType,
				typ:               int(mpegConf.Type),
				sampleRate:        mpegConf.SampleRate,
				channelCount:      mpegConf.ChannelCount,
				aotSpecificConfig: mpegConf.AOTSpecificConfig,
				mpegConf:          enc,
			}, nil
		}
	}

	return nil, fmt.Errorf("config is missing (%v)", v)
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

func (t *TrackAAC) clone() Track {
	return &TrackAAC{
		control:           t.control,
		payloadType:       t.payloadType,
		typ:               t.typ,
		sampleRate:        t.sampleRate,
		channelCount:      t.channelCount,
		aotSpecificConfig: t.aotSpecificConfig,
		mpegConf:          t.mpegConf,
	}
}

func (t *TrackAAC) getControl() string {
	return t.control
}

func (t *TrackAAC) setControl(c string) {
	t.control = c
}

func (t *TrackAAC) url(contentBase *base.URL) (*base.URL, error) {
	return trackURL(t, contentBase)
}

func (t *TrackAAC) mediaDescription() *psdp.MediaDescription {
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
					"sizelength=13; " +
					"indexlength=3; " +
					"indexdeltalength=3; " +
					"config=" + hex.EncodeToString(t.mpegConf),
			},
			{
				Key:   "control",
				Value: t.control,
			},
		},
	}
}
