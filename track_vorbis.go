package gortsplib

import (
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"sync"

	psdp "github.com/pion/sdp/v3"
)

// TrackVorbis is a Vorbis track.
type TrackVorbis struct {
	PayloadType   uint8
	SampleRate    int
	ChannelCount  int
	Configuration []byte

	trackBase
	mutex sync.RWMutex
}

func newTrackVorbisFromMediaDescription(
	control string,
	payloadType uint8,
	clock string,
	md *psdp.MediaDescription,
) (*TrackVorbis, error) {
	tmp := strings.SplitN(clock, "/", 32)
	if len(tmp) != 2 {
		return nil, fmt.Errorf("invalid clock (%v)", clock)
	}

	sampleRate, err := strconv.ParseInt(tmp[0], 10, 64)
	if err != nil {
		return nil, err
	}

	channelCount, err := strconv.ParseInt(tmp[1], 10, 64)
	if err != nil {
		return nil, err
	}

	t := &TrackVorbis{
		PayloadType:  payloadType,
		SampleRate:   int(sampleRate),
		ChannelCount: int(channelCount),
		trackBase: trackBase{
			control: control,
		},
	}

	v, ok := md.Attribute("fmtp")
	if !ok {
		return nil, fmt.Errorf("fmtp attribute is missing")
	}

	tmp = strings.SplitN(v, " ", 2)
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

		if tmp[0] == "configuration" {
			conf, err := base64.StdEncoding.DecodeString(tmp[1])
			if err != nil {
				return nil, fmt.Errorf("invalid AAC config (%v)", tmp[1])
			}

			t.Configuration = conf
		}
	}

	if t.Configuration == nil {
		return nil, fmt.Errorf("config is missing (%v)", v)
	}

	return t, nil
}

// ClockRate returns the track clock rate.
func (t *TrackVorbis) ClockRate() int {
	return t.SampleRate
}

// MediaDescription returns the track media description in SDP format.
func (t *TrackVorbis) MediaDescription() *psdp.MediaDescription {
	t.mutex.RLock()
	defer t.mutex.RUnlock()

	typ := strconv.FormatInt(int64(t.PayloadType), 10)

	fmtp := typ + " configuration=" + base64.StdEncoding.EncodeToString(t.Configuration)

	return &psdp.MediaDescription{
		MediaName: psdp.MediaName{
			Media:   "audio",
			Protos:  []string{"RTP", "AVP"},
			Formats: []string{typ},
		},
		Attributes: []psdp.Attribute{
			{
				Key: "rtpmap",
				Value: typ + " VORBIS/" + strconv.FormatInt(int64(t.SampleRate), 10) +
					"/" + strconv.FormatInt(int64(t.ChannelCount), 10),
			},
			{
				Key:   "fmtp",
				Value: fmtp,
			},
			{
				Key:   "control",
				Value: t.control,
			},
		},
	}
}

func (t *TrackVorbis) clone() Track {
	return &TrackVorbis{
		PayloadType:   t.PayloadType,
		SampleRate:    t.SampleRate,
		ChannelCount:  t.ChannelCount,
		Configuration: t.Configuration,
		trackBase:     t.trackBase,
	}
}
