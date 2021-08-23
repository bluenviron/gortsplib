package rtpaac

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

var sampleRates = []int{
	96000,
	88200,
	64000,
	48000,
	44100,
	32000,
	24000,
	22050,
	16000,
	12000,
	11025,
	8000,
	7350,
}

var channelCounts = []int{
	1,
	2,
	3,
	4,
	5,
	6,
	8,
}

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

	switch {
	case sampleRateIndex <= 12:
		c.SampleRate = sampleRates[sampleRateIndex]

	case sampleRateIndex == 15:
		tmp, err := r.ReadBits(24)
		if err != nil {
			return err
		}
		c.SampleRate = int(tmp)

	default:
		return fmt.Errorf("invalid sample rate index (%d)", sampleRateIndex)
	}

	channelConfig, err := r.ReadBits(4)
	if err != nil {
		return err
	}

	switch {
	case channelConfig == 0:
		return fmt.Errorf("not yet supported")

	case channelConfig >= 1 && channelConfig <= 7:
		c.ChannelCount = channelCounts[channelConfig-1]

	default:
		return fmt.Errorf("invalid channel configuration (%d)", channelConfig)
	}

	return nil
}
