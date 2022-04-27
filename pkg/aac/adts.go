package aac

import (
	"fmt"
)

// ADTSPacket is an ADTS packet
type ADTSPacket struct {
	Type         int
	SampleRate   int
	ChannelCount int
	AU           []byte
}

// DecodeADTS decodes an ADTS stream into ADTS packets.
func DecodeADTS(byts []byte) ([]*ADTSPacket, error) {
	// refs: https://wiki.multimedia.cx/index.php/ADTS

	var ret []*ADTSPacket

	for {
		bl := len(byts)

		if bl == 0 {
			break
		}

		if bl < 8 {
			return nil, fmt.Errorf("invalid length")
		}

		syncWord := (uint16(byts[0]) << 4) | (uint16(byts[1]) >> 4)
		if syncWord != 0xfff {
			return nil, fmt.Errorf("invalid syncword")
		}

		protectionAbsent := byts[1] & 0x01
		if protectionAbsent != 1 {
			return nil, fmt.Errorf("CRC is not supported")
		}

		pkt := &ADTSPacket{}

		pkt.Type = int((byts[2] >> 6) + 1)

		switch MPEG4AudioType(pkt.Type) {
		case MPEG4AudioTypeAACLC:
		default:
			return nil, fmt.Errorf("unsupported audio type: %d", pkt.Type)
		}

		sampleRateIndex := (byts[2] >> 2) & 0x0F

		switch {
		case sampleRateIndex <= 12:
			pkt.SampleRate = sampleRates[sampleRateIndex]

		default:
			return nil, fmt.Errorf("invalid sample rate index: %d", sampleRateIndex)
		}

		channelConfig := ((byts[2] & 0x01) << 2) | ((byts[3] >> 6) & 0x03)

		switch {
		case channelConfig >= 1 && channelConfig <= 6:
			pkt.ChannelCount = int(channelConfig)

		case channelConfig == 7:
			pkt.ChannelCount = 8

		default:
			return nil, fmt.Errorf("invalid channel configuration: %d", channelConfig)
		}

		frameLen := int(((uint16(byts[3])&0x03)<<11)|
			(uint16(byts[4])<<3)|
			((uint16(byts[5])>>5)&0x07)) - 7

		// fullness := ((uint16(byts[5]) & 0x1F) << 6) | ((uint16(byts[6]) >> 2) & 0x3F)

		frameCount := byts[6] & 0x03
		if frameCount != 0 {
			return nil, fmt.Errorf("multiple frame count not supported")
		}

		if len(byts[7:]) < frameLen {
			return nil, fmt.Errorf("invalid frame length")
		}

		pkt.AU = byts[7 : 7+frameLen]
		byts = byts[7+frameLen:]

		ret = append(ret, pkt)
	}

	return ret, nil
}

// EncodeADTS encodes ADTS packets into an ADTS stream.
func EncodeADTS(pkts []*ADTSPacket) ([]byte, error) {
	var ret []byte

	for _, pkt := range pkts {
		sampleRateIndex, ok := reverseSampleRates[pkt.SampleRate]
		if !ok {
			return nil, fmt.Errorf("invalid sample rate: %d", pkt.SampleRate)
		}

		var channelConfig int
		switch {
		case pkt.ChannelCount >= 1 && pkt.ChannelCount <= 6:
			channelConfig = pkt.ChannelCount

		case pkt.ChannelCount == 8:
			channelConfig = 7

		default:
			return nil, fmt.Errorf("invalid channel count (%d)", pkt.ChannelCount)
		}

		frameLen := len(pkt.AU) + 7

		fullness := 0x07FF // like ffmpeg does

		header := make([]byte, 7)
		header[0] = 0xFF
		header[1] = 0xF1
		header[2] = uint8(((pkt.Type - 1) << 6) | (sampleRateIndex << 2) | ((channelConfig >> 2) & 0x01))
		header[3] = uint8((channelConfig&0x03)<<6 | (frameLen>>11)&0x03)
		header[4] = uint8((frameLen >> 3) & 0xFF)
		header[5] = uint8((frameLen&0x07)<<5 | ((fullness >> 6) & 0x1F))
		header[6] = uint8((fullness & 0x3F) << 2)
		ret = append(ret, header...)

		ret = append(ret, pkt.AU...)
	}

	return ret, nil
}
