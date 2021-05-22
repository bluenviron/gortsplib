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

// MPEG4AudioConfig is a MPEG-4 Audio configuration.
type MPEG4AudioConfig struct {
	Type         MPEG4AudioType
	SampleRate   int
	ChannelCount int
}

var sampleRates = map[uint64]int{
	0:  96000,
	1:  88200,
	2:  64000,
	3:  48000,
	4:  44100,
	5:  32000,
	6:  24000,
	7:  22050,
	8:  16000,
	9:  12000,
	10: 11025,
	11: 8000,
	12: 7350,
}

var channelCounts = map[uint64]int{
	1: 1,
	2: 2,
	3: 3,
	4: 4,
	5: 5,
	6: 6,
	7: 8,
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
		c.ChannelCount = channelCounts[channelConfig]

	default:
		return fmt.Errorf("invalid channel configuration: %d", channelConfig)
	}

	return nil
}
