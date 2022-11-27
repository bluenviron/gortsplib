package gortsplib

import (
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	"github.com/pion/rtp"

	"github.com/aler9/gortsplib/pkg/mpeg4audio"
	"github.com/aler9/gortsplib/pkg/rtpcodecs/rtpmpeg4audio"
)

// TrackMPEG4Audio is a MPEG-4 audio track.
type TrackMPEG4Audio struct {
	PayloadType      uint8
	Config           *mpeg4audio.Config
	SizeLength       int
	IndexLength      int
	IndexDeltaLength int
}

// String returns a description of the track.
func (t *TrackMPEG4Audio) String() string {
	return "MPEG4-audio"
}

// ClockRate returns the clock rate.
func (t *TrackMPEG4Audio) ClockRate() int {
	return t.Config.SampleRate
}

// GetPayloadType returns the payload type.
func (t *TrackMPEG4Audio) GetPayloadType() uint8 {
	return t.PayloadType
}

func (t *TrackMPEG4Audio) unmarshal(payloadType uint8, clock string, codec string, rtpmap string, fmtp string) error {
	t.PayloadType = payloadType

	if fmtp == "" {
		return fmt.Errorf("fmtp attribute is missing")
	}

	for _, kv := range strings.Split(fmtp, ";") {
		kv = strings.Trim(kv, " ")

		if len(kv) == 0 {
			continue
		}

		tmp := strings.SplitN(kv, "=", 2)
		if len(tmp) != 2 {
			return fmt.Errorf("invalid fmtp (%v)", fmtp)
		}

		switch strings.ToLower(tmp[0]) {
		case "config":
			enc, err := hex.DecodeString(tmp[1])
			if err != nil {
				return fmt.Errorf("invalid AAC config (%v)", tmp[1])
			}

			t.Config = &mpeg4audio.Config{}
			err = t.Config.Unmarshal(enc)
			if err != nil {
				return fmt.Errorf("invalid AAC config (%v)", tmp[1])
			}

		case "sizelength":
			val, err := strconv.ParseUint(tmp[1], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid AAC SizeLength (%v)", tmp[1])
			}
			t.SizeLength = int(val)

		case "indexlength":
			val, err := strconv.ParseUint(tmp[1], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid AAC IndexLength (%v)", tmp[1])
			}
			t.IndexLength = int(val)

		case "indexdeltalength":
			val, err := strconv.ParseUint(tmp[1], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid AAC IndexDeltaLength (%v)", tmp[1])
			}
			t.IndexDeltaLength = int(val)
		}
	}

	if t.Config == nil {
		return fmt.Errorf("config is missing (%v)", fmtp)
	}

	if t.SizeLength == 0 {
		return fmt.Errorf("sizelength is missing (%v)", fmtp)
	}

	return nil
}

func (t *TrackMPEG4Audio) marshal() (string, string) {
	enc, err := t.Config.Marshal()
	if err != nil {
		return "", ""
	}

	sampleRate := t.Config.SampleRate
	if t.Config.ExtensionSampleRate != 0 {
		sampleRate = t.Config.ExtensionSampleRate
	}

	fmtp := "profile-level-id=1; " +
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
		"config=" + hex.EncodeToString(enc)

	return "mpeg4-generic/" + strconv.FormatInt(int64(sampleRate), 10) +
		"/" + strconv.FormatInt(int64(t.Config.ChannelCount), 10), fmtp
}

func (t *TrackMPEG4Audio) clone() Track {
	return &TrackMPEG4Audio{
		PayloadType:      t.PayloadType,
		Config:           t.Config,
		SizeLength:       t.SizeLength,
		IndexLength:      t.IndexLength,
		IndexDeltaLength: t.IndexDeltaLength,
	}
}

func (t *TrackMPEG4Audio) ptsEqualsDTS(*rtp.Packet) bool {
	return true
}

// CreateDecoder creates a decoder able to decode the content of the track.
func (t *TrackMPEG4Audio) CreateDecoder() *rtpmpeg4audio.Decoder {
	d := &rtpmpeg4audio.Decoder{
		SampleRate:       t.Config.SampleRate,
		SizeLength:       t.SizeLength,
		IndexLength:      t.IndexLength,
		IndexDeltaLength: t.IndexDeltaLength,
	}
	d.Init()
	return d
}

// CreateEncoder creates an encoder able to encode the content of the track.
func (t *TrackMPEG4Audio) CreateEncoder() *rtpmpeg4audio.Encoder {
	e := &rtpmpeg4audio.Encoder{
		PayloadType:      t.PayloadType,
		SampleRate:       t.Config.SampleRate,
		SizeLength:       t.SizeLength,
		IndexLength:      t.IndexLength,
		IndexDeltaLength: t.IndexDeltaLength,
	}
	e.Init()
	return e
}
