package rtpmjpeg

import (
	"crypto/rand"
	"fmt"

	"github.com/bluenviron/mediacommon/v2/pkg/codecs/jpeg"
	"github.com/pion/rtp"
)

const (
	rtpVersion            = 2
	defaultPayloadMaxSize = 1450 // 1500 (UDP MTU) - 20 (IP header) - 8 (UDP header) - 12 (RTP header) - 10 (SRTP overhead)
	payloadType           = 26
)

func randUint32() (uint32, error) {
	var b [4]byte
	_, err := rand.Read(b[:])
	if err != nil {
		return 0, err
	}
	return uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3]), nil
}

// Encoder is a RTP/M-JPEG encoder.
// Specification: RFC2435
type Encoder struct {
	// SSRC of packets (optional).
	// It defaults to a random value.
	SSRC *uint32

	// initial sequence number of packets (optional).
	// It defaults to a random value.
	InitialSequenceNumber *uint16

	// maximum size of packet payloads (optional).
	// It defaults to 1450.
	PayloadMaxSize int

	sequenceNumber uint16
}

// Init initializes the encoder.
func (e *Encoder) Init() error {
	if e.SSRC == nil {
		v, err := randUint32()
		if err != nil {
			return err
		}
		e.SSRC = &v
	}
	if e.InitialSequenceNumber == nil {
		v, err := randUint32()
		if err != nil {
			return err
		}
		e.InitialSequenceNumber = new(uint16(v))
	}
	if e.PayloadMaxSize == 0 {
		e.PayloadMaxSize = defaultPayloadMaxSize
	}

	e.sequenceNumber = *e.InitialSequenceNumber
	return nil
}

// Encode encodes an image into RTP/M-JPEG packets.
// Image must be a valid JPEG image and satisfy the RTP/M-JPEG constraints
// (width and height must be less than 2040 and must be a multiple of 8).
// The method might panic otherwise.
func (e *Encoder) Encode(image []byte) ([]*rtp.Packet, error) {
	image = image[2:]
	var sof *jpeg.StartOfFrame1
	var dri *jpeg.DefineRestartInterval
	quantizationTables := make(map[uint8][]byte)
	var tableIDs [3]uint8
	var data []byte

outer:
	for len(image) >= 2 {
		h1 := image[1]
		image = image[2:]

		switch h1 {
		case 0xE0, 0xE1, 0xE2, // JFIF
			jpeg.MarkerDefineHuffmanTable,
			jpeg.MarkerComment:
			mlen := int(image[0])<<8 | int(image[1])
			image = image[mlen:]

		case jpeg.MarkerDefineQuantizationTable:
			mlen := int(image[0])<<8 | int(image[1])

			var dqt jpeg.DefineQuantizationTable
			err := dqt.Unmarshal(image[2:mlen])
			if err != nil {
				return nil, err
			}
			image = image[mlen:]

			for _, t := range dqt.Tables {
				quantizationTables[t.ID] = t.Data
			}

		case jpeg.MarkerDefineRestartInterval:
			mlen := int(image[0])<<8 | int(image[1])

			dri = &jpeg.DefineRestartInterval{}
			err := dri.Unmarshal(image[2:mlen])
			if err != nil {
				return nil, err
			}
			image = image[mlen:]

		case jpeg.MarkerStartOfFrame1:
			mlen := int(image[0])<<8 | int(image[1])

			sof = &jpeg.StartOfFrame1{}
			sofData := image[2:mlen]
			err := sof.Unmarshal(sofData)
			if err != nil {
				return nil, err
			}
			tableIDs = [3]uint8{sofData[8], sofData[11], sofData[14]}
			image = image[mlen:]

		case jpeg.MarkerStartOfScan:
			mlen := int(image[0])<<8 | int(image[1])

			var sos jpeg.StartOfScan
			err := sos.Unmarshal(image[2:mlen])
			if err != nil {
				return nil, err
			}
			image = image[mlen:]

			data = image
			break outer
		}
	}

	if tableIDs[1] != tableIDs[2] {
		return nil, fmt.Errorf("chroma components use different quantization tables")
	}

	lumaTable, ok := quantizationTables[tableIDs[0]]
	if !ok {
		return nil, fmt.Errorf("luminance quantization table %d is missing", tableIDs[0])
	}

	chromaTable, ok := quantizationTables[tableIDs[1]]
	if !ok {
		return nil, fmt.Errorf("chrominance quantization table %d is missing", tableIDs[1])
	}

	jh := headerJPEG{
		TypeSpecific: 0,
		Type:         sof.Type,
		Quantization: 255,
		Width:        sof.Width,
		Height:       sof.Height,
	}

	if dri != nil {
		if dri.Interval == 0 {
			return nil, fmt.Errorf("restart interval must not be zero")
		}
		jh.Type += 64
	}

	first := true
	offset := 0
	var ret []*rtp.Packet

	for {
		var buf []byte

		jh.FragmentOffset = uint32(offset)
		buf = jh.marshal(buf)

		if dri != nil {
			buf = headerRestartMarker{
				Interval: dri.Interval,
				Count:    0xFFFF,
			}.marshal(buf)
		}

		if first {
			first = false

			buf = (headerQuantizationTable{
				Tables: [][]byte{lumaTable, chromaTable},
			}).marshal(buf)
		}

		remaining := e.PayloadMaxSize - len(buf)
		ldata := len(data)
		if remaining > ldata {
			remaining = ldata
		}

		buf = append(buf, data[:remaining]...)
		data = data[remaining:]
		offset += remaining

		ret = append(ret, &rtp.Packet{
			Header: rtp.Header{
				Version:        rtpVersion,
				PayloadType:    payloadType,
				SequenceNumber: e.sequenceNumber,
				SSRC:           *e.SSRC,
				Marker:         len(data) == 0,
			},
			Payload: buf,
		})
		e.sequenceNumber++

		if len(data) == 0 {
			break
		}
	}

	return ret, nil
}
