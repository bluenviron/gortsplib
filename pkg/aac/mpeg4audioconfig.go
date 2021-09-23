package aac

import (
	"bytes"
	"fmt"
	"io"

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
	Type              MPEG4AudioType
	SampleRate        int
	ChannelCount      int
	AOTSpecificConfig []byte
}

// Decode decodes an MPEG4AudioConfig.
func (c *MPEG4AudioConfig) Decode(byts []byte) error {
	// ref: https://wiki.multimedia.cx/index.php/MPEG-4_Audio

	r := bitio.NewReader(bytes.NewBuffer(byts))

	tmp, err := r.ReadBits(5)
	if err != nil {
		return err
	}
	c.Type = MPEG4AudioType(tmp)

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

	for {
		byt, err := r.ReadBits(8)
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}

		c.AOTSpecificConfig = append(c.AOTSpecificConfig, uint8(byt))
	}

	return nil
}

// Encode encodes an MPEG4AudioConfig.
func (c MPEG4AudioConfig) Encode() ([]byte, error) {
	var buf bytes.Buffer
	w := bitio.NewWriter(&buf)

	w.WriteBits(uint64(c.Type), 5)

	sampleRateIndex := func() int {
		for i, s := range sampleRates {
			if s == c.SampleRate {
				return i
			}
		}
		return -1
	}()

	if sampleRateIndex != -1 {
		w.WriteBits(uint64(sampleRateIndex), 4)
	} else {
		w.WriteBits(uint64(15), 4)
		w.WriteBits(uint64(c.SampleRate), 24)
	}

	channelConfig := func() int {
		for i, co := range channelCounts {
			if co == c.ChannelCount {
				return i + 1
			}
		}
		return -1
	}()

	if channelConfig == -1 {
		return nil, fmt.Errorf("invalid channel count (%d)", c.ChannelCount)
	}

	w.WriteBits(uint64(channelConfig), 4)

	for _, b := range c.AOTSpecificConfig {
		w.WriteBits(uint64(b), 8)
	}

	w.Close()

	return buf.Bytes(), nil
}
