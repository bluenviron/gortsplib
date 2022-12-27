package rtpmjpeg

import (
	"errors"
	"fmt"
	"time"

	"github.com/pion/rtp"

	"github.com/aler9/gortsplib/v2/pkg/formatdecenc/rtpmjpeg/headers"
	"github.com/aler9/gortsplib/v2/pkg/formatdecenc/rtpmjpeg/jpeg"
	"github.com/aler9/gortsplib/v2/pkg/rtptimedec"
)

// ErrMorePacketsNeeded is returned when more packets are needed.
var ErrMorePacketsNeeded = errors.New("need more packets")

// ErrNonStartingPacketAndNoPrevious is returned when we received a non-starting
// fragment of an image and we didn't received anything before.
// It's normal to receive this when we are decoding a stream that has been already
// running for some time.
var ErrNonStartingPacketAndNoPrevious = errors.New(
	"received a non-starting fragment without any previous starting fragment")

var lumDcCodeLens = []byte{
	0, 1, 5, 1, 1, 1, 1, 1, 1, 0, 0, 0, 0, 0, 0, 0,
}

var lumDcSymbols = []byte{
	0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11,
}

var lumAcCodelens = []byte{
	0, 2, 1, 3, 3, 2, 4, 3, 5, 5, 4, 4, 0, 0, 1, 0x7d,
}

var lumAcSymbols = []byte{ //nolint:dupl
	0x01, 0x02, 0x03, 0x00, 0x04, 0x11, 0x05, 0x12,
	0x21, 0x31, 0x41, 0x06, 0x13, 0x51, 0x61, 0x07,
	0x22, 0x71, 0x14, 0x32, 0x81, 0x91, 0xa1, 0x08,
	0x23, 0x42, 0xb1, 0xc1, 0x15, 0x52, 0xd1, 0xf0,
	0x24, 0x33, 0x62, 0x72, 0x82, 0x09, 0x0a, 0x16,
	0x17, 0x18, 0x19, 0x1a, 0x25, 0x26, 0x27, 0x28,
	0x29, 0x2a, 0x34, 0x35, 0x36, 0x37, 0x38, 0x39,
	0x3a, 0x43, 0x44, 0x45, 0x46, 0x47, 0x48, 0x49,
	0x4a, 0x53, 0x54, 0x55, 0x56, 0x57, 0x58, 0x59,
	0x5a, 0x63, 0x64, 0x65, 0x66, 0x67, 0x68, 0x69,
	0x6a, 0x73, 0x74, 0x75, 0x76, 0x77, 0x78, 0x79,
	0x7a, 0x83, 0x84, 0x85, 0x86, 0x87, 0x88, 0x89,
	0x8a, 0x92, 0x93, 0x94, 0x95, 0x96, 0x97, 0x98,
	0x99, 0x9a, 0xa2, 0xa3, 0xa4, 0xa5, 0xa6, 0xa7,
	0xa8, 0xa9, 0xaa, 0xb2, 0xb3, 0xb4, 0xb5, 0xb6,
	0xb7, 0xb8, 0xb9, 0xba, 0xc2, 0xc3, 0xc4, 0xc5,
	0xc6, 0xc7, 0xc8, 0xc9, 0xca, 0xd2, 0xd3, 0xd4,
	0xd5, 0xd6, 0xd7, 0xd8, 0xd9, 0xda, 0xe1, 0xe2,
	0xe3, 0xe4, 0xe5, 0xe6, 0xe7, 0xe8, 0xe9, 0xea,
	0xf1, 0xf2, 0xf3, 0xf4, 0xf5, 0xf6, 0xf7, 0xf8,
	0xf9, 0xfa,
}

var chmDcCodelens = []byte{
	0, 3, 1, 1, 1, 1, 1, 1, 1, 1, 1, 0, 0, 0, 0, 0,
}

var chmDcSymbols = []byte{
	0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11,
}

var chmAcCodelens = []byte{
	0, 2, 1, 2, 4, 4, 3, 4, 7, 5, 4, 4, 0, 1, 2, 0x77,
}

var chmAcSymbols = []byte{ //nolint:dupl
	0x00, 0x01, 0x02, 0x03, 0x11, 0x04, 0x05, 0x21,
	0x31, 0x06, 0x12, 0x41, 0x51, 0x07, 0x61, 0x71,
	0x13, 0x22, 0x32, 0x81, 0x08, 0x14, 0x42, 0x91,
	0xa1, 0xb1, 0xc1, 0x09, 0x23, 0x33, 0x52, 0xf0,
	0x15, 0x62, 0x72, 0xd1, 0x0a, 0x16, 0x24, 0x34,
	0xe1, 0x25, 0xf1, 0x17, 0x18, 0x19, 0x1a, 0x26,
	0x27, 0x28, 0x29, 0x2a, 0x35, 0x36, 0x37, 0x38,
	0x39, 0x3a, 0x43, 0x44, 0x45, 0x46, 0x47, 0x48,
	0x49, 0x4a, 0x53, 0x54, 0x55, 0x56, 0x57, 0x58,
	0x59, 0x5a, 0x63, 0x64, 0x65, 0x66, 0x67, 0x68,
	0x69, 0x6a, 0x73, 0x74, 0x75, 0x76, 0x77, 0x78,
	0x79, 0x7a, 0x82, 0x83, 0x84, 0x85, 0x86, 0x87,
	0x88, 0x89, 0x8a, 0x92, 0x93, 0x94, 0x95, 0x96,
	0x97, 0x98, 0x99, 0x9a, 0xa2, 0xa3, 0xa4, 0xa5,
	0xa6, 0xa7, 0xa8, 0xa9, 0xaa, 0xb2, 0xb3, 0xb4,
	0xb5, 0xb6, 0xb7, 0xb8, 0xb9, 0xba, 0xc2, 0xc3,
	0xc4, 0xc5, 0xc6, 0xc7, 0xc8, 0xc9, 0xca, 0xd2,
	0xd3, 0xd4, 0xd5, 0xd6, 0xd7, 0xd8, 0xd9, 0xda,
	0xe2, 0xe3, 0xe4, 0xe5, 0xe6, 0xe7, 0xe8, 0xe9,
	0xea, 0xf2, 0xf3, 0xf4, 0xf5, 0xf6, 0xf7, 0xf8,
	0xf9, 0xfa,
}

// Decoder is a RTP/MJPEG decoder.
type Decoder struct {
	timeDecoder         *rtptimedec.Decoder
	firstPacketReceived bool
	fragmentedSize      int
	fragments           [][]byte
	firstJpegHeader     *headers.JPEG
	firstQTHeader       *headers.QuantizationTable
}

// Init initializes the decoder.
func (d *Decoder) Init() {
	d.timeDecoder = rtptimedec.New(rtpClockRate)
}

// Decode decodes an image from a RTP/MJPEG packet.
func (d *Decoder) Decode(pkt *rtp.Packet) ([]byte, time.Duration, error) {
	byts := pkt.Payload

	var jh headers.JPEG
	n, err := jh.Unmarshal(byts)
	if err != nil {
		return nil, 0, err
	}
	byts = byts[n:]

	if jh.Width > maxDimension {
		return nil, 0, fmt.Errorf("Width of %d is not supported", jh.Width)
	}

	if jh.Height > maxDimension {
		return nil, 0, fmt.Errorf("Height of %d is not supported", jh.Height)
	}

	if jh.FragmentOffset == 0 {
		if jh.Quantization >= 128 {
			d.firstQTHeader = &headers.QuantizationTable{}
			n, err := d.firstQTHeader.Unmarshal(byts)
			if err != nil {
				return nil, 0, err
			}
			byts = byts[n:]
		} else {
			d.firstQTHeader = nil
		}

		d.fragments = d.fragments[:0] // discard pending fragmented packets
		d.fragmentedSize = len(byts)
		d.fragments = append(d.fragments, byts)
		d.firstJpegHeader = &jh
		d.firstPacketReceived = true
	} else {
		if len(d.fragments) == 0 {
			if !d.firstPacketReceived {
				return nil, 0, ErrNonStartingPacketAndNoPrevious
			}

			return nil, 0, fmt.Errorf("received a non-starting fragment")
		}

		if int(jh.FragmentOffset) != d.fragmentedSize {
			d.fragments = d.fragments[:0] // discard pending fragmented packets
			return nil, 0, fmt.Errorf("received wrong fragment")
		}

		d.fragmentedSize += len(byts)
		d.fragments = append(d.fragments, byts)
	}

	if !pkt.Marker {
		return nil, 0, ErrMorePacketsNeeded
	}

	data := make([]byte, d.fragmentedSize)
	pos := 0

	for _, frag := range d.fragments {
		pos += copy(data[pos:], frag)
	}

	d.fragments = d.fragments[:0]

	var buf []byte

	buf = jpeg.StartOfImage{}.Marshal(buf)

	var dqt jpeg.DefineQuantizationTable
	id := uint8(0)
	for i := 0; i < len(d.firstQTHeader.Tables); i += 64 {
		dqt.Tables = append(dqt.Tables, jpeg.QuantizationTable{
			ID:   id,
			Data: d.firstQTHeader.Tables[i : i+64],
		})
		id++
	}
	buf = dqt.Marshal(buf)

	buf = jpeg.StartOfFrame1{
		Type:                   d.firstJpegHeader.Type,
		Width:                  d.firstJpegHeader.Width,
		Height:                 d.firstJpegHeader.Height,
		QuantizationTableCount: id,
	}.Marshal(buf)

	buf = jpeg.DefineHuffmanTable{
		Codes:       lumDcCodeLens,
		Symbols:     lumDcSymbols,
		TableNumber: 0,
		TableClass:  0,
	}.Marshal(buf)

	buf = jpeg.DefineHuffmanTable{
		Codes:       lumAcCodelens,
		Symbols:     lumAcSymbols,
		TableNumber: 0,
		TableClass:  1,
	}.Marshal(buf)

	buf = jpeg.DefineHuffmanTable{
		Codes:       chmDcCodelens,
		Symbols:     chmDcSymbols,
		TableNumber: 1,
		TableClass:  0,
	}.Marshal(buf)

	buf = jpeg.DefineHuffmanTable{
		Codes:       chmAcCodelens,
		Symbols:     chmAcSymbols,
		TableNumber: 1,
		TableClass:  1,
	}.Marshal(buf)

	buf = jpeg.StartOfScan{}.Marshal(buf)

	buf = append(buf, data...)

	if data[len(data)-2] != 0xFF || data[len(data)-1] != jpeg.MarkerEndOfImage {
		buf = append(buf, []byte{0xFF, jpeg.MarkerEndOfImage}...)
	}

	return buf, d.timeDecoder.Decode(pkt.Timestamp), nil
}
