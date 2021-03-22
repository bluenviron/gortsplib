// Package codecaac contains utilities to deal with the AAC codec.
package codecaac

import (
	"bytes"
	"fmt"

	"github.com/icza/bitio"
)

// MPEG4AudioType is the type of a MPEG-4 Audio stream.
type MPEG4AudioType int

// standard MPEG-4 Audio types.
const (
	MPEG4AudioTypeAACLC MPEG4AudioType = 2
)

// MPEG4AudioConfig is a MPEG-4 Audio configuration.
type MPEG4AudioConfig struct {
	Type         MPEG4AudioType
	SampleRate   int
	ChannelCount int
}

// Decode decodes an MPEG-4 Audio configuration.
func (c *MPEG4AudioConfig) Decode(byts []byte) error {
	// ref: https://wiki.multimedia.cx/index.php/MPEG-4_Audio

	r := bitio.NewReader(bytes.NewBuffer(byts))

	tmp, err := r.ReadBits(5)
	if err != nil {
		return err
	}
	c.Type = MPEG4AudioType(tmp)

	if tmp == 31 {
		tmp, err = r.ReadBits(6)
		if err != nil {
			return err
		}
		c.Type = MPEG4AudioType(tmp + 32)
	}

	switch c.Type {
	case MPEG4AudioTypeAACLC:
	default:
		return fmt.Errorf("unsupported type: %d", c.Type)
	}

	sampleRateIndex, err := r.ReadBits(4)
	if err != nil {
		return err
	}

	switch sampleRateIndex {
	case 0:
		c.SampleRate = 96000
	case 1:
		c.SampleRate = 88200
	case 2:
		c.SampleRate = 64000
	case 3:
		c.SampleRate = 48000
	case 4:
		c.SampleRate = 44100
	case 5:
		c.SampleRate = 32000
	case 6:
		c.SampleRate = 24000
	case 7:
		c.SampleRate = 22050
	case 8:
		c.SampleRate = 16000
	case 9:
		c.SampleRate = 12000
	case 10:
		c.SampleRate = 11025
	case 11:
		c.SampleRate = 8000
	case 12:
		c.SampleRate = 7350

	case 15:
		sampleRateIndex, err := r.ReadBits(24)
		if err != nil {
			return err
		}
		c.SampleRate = int(sampleRateIndex)

	default:
		return fmt.Errorf("invalid sample rate index: %d", sampleRateIndex)
	}

	channelConfig, err := r.ReadBits(4)
	if err != nil {
		return err
	}

	switch channelConfig {
	case 0:
		return fmt.Errorf("not yet supported")

	case 1:
		c.ChannelCount = 1
	case 2:
		c.ChannelCount = 2
	case 3:
		c.ChannelCount = 3
	case 4:
		c.ChannelCount = 4
	case 5:
		c.ChannelCount = 5
	case 6:
		c.ChannelCount = 6
	case 7:
		c.ChannelCount = 8

	default:
		return fmt.Errorf("invalid channel configuration: %d", channelConfig)
	}

	return nil
}
