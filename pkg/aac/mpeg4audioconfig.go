package aac

import (
	"bytes"
	"fmt"
	"io"

	"github.com/icza/bitio"
)

// MPEG4AudioConfig is a MPEG-4 Audio configuration.
type MPEG4AudioConfig struct {
	Type              MPEG4AudioType
	SampleRate        int
	ChannelCount      int
	AOTSpecificConfig []byte
}

// Unmarshal decodes an MPEG4AudioConfig.
func (c *MPEG4AudioConfig) Unmarshal(byts []byte) error {
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

	case channelConfig >= 1 && channelConfig <= 6:
		c.ChannelCount = int(channelConfig)

	case channelConfig == 7:
		c.ChannelCount = 8

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

func (c MPEG4AudioConfig) marshalSize() int {
	n := 5 + 4 + len(c.AOTSpecificConfig)*8
	_, ok := reverseSampleRates[c.SampleRate]
	if !ok {
		n += 28
	} else {
		n += 4
	}

	ret := n / 8
	if n%8 != 0 {
		ret++
	}
	return ret
}

// Marshal encodes an MPEG4AudioConfig.
func (c MPEG4AudioConfig) Marshal() ([]byte, error) {
	buf := make([]byte, c.marshalSize())
	w := bitio.NewWriter(bytes.NewBuffer(buf[:0]))

	w.WriteBits(uint64(c.Type), 5)

	sampleRateIndex, ok := reverseSampleRates[c.SampleRate]
	if !ok {
		w.WriteBits(uint64(15), 4)
		w.WriteBits(uint64(c.SampleRate), 24)
	} else {
		w.WriteBits(uint64(sampleRateIndex), 4)
	}

	var channelConfig int
	switch {
	case c.ChannelCount >= 1 && c.ChannelCount <= 6:
		channelConfig = c.ChannelCount

	case c.ChannelCount == 8:
		channelConfig = 7

	default:
		return nil, fmt.Errorf("invalid channel count (%d)", c.ChannelCount)
	}

	w.WriteBits(uint64(channelConfig), 4)

	for _, b := range c.AOTSpecificConfig {
		w.WriteBits(uint64(b), 8)
	}

	w.Close()

	return buf, nil
}
