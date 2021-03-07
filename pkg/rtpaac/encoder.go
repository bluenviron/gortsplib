// Package rtpaac contains a RTP/AAC decoder and encoder.
package rtpaac

import (
	"encoding/binary"
	"fmt"
	"math/rand"

	"github.com/pion/rtp"
)

const (
	rtpVersion        = 0x02
	rtpPayloadMaxSize = 1460 // 1500 (mtu) - 20 (ip header) - 8 (udp header) - 12 (rtp header)
)

// Encoder is a RPT/AAC encoder.
type Encoder struct {
	payloadType    uint8
	clockRate      float64
	sequenceNumber uint16
	ssrc           uint32
	initialTs      uint32
}

// NewEncoder allocates an Encoder.
func NewEncoder(payloadType uint8,
	clockRate int,
	sequenceNumber *uint16,
	ssrc *uint32,
	initialTs *uint32) *Encoder {
	return &Encoder{
		payloadType: payloadType,
		clockRate:   float64(clockRate),
		sequenceNumber: func() uint16 {
			if sequenceNumber != nil {
				return *sequenceNumber
			}
			return uint16(rand.Uint32())
		}(),
		ssrc: func() uint32 {
			if ssrc != nil {
				return *ssrc
			}
			return rand.Uint32()
		}(),
		initialTs: func() uint32 {
			if initialTs != nil {
				return *initialTs
			}
			return rand.Uint32()
		}(),
	}
}

// Encode encodes an AU into an RTP/AAC packet.
func (e *Encoder) Encode(at *AUAndTimestamp) ([]byte, error) {
	if len(at.AU) > rtpPayloadMaxSize {
		return nil, fmt.Errorf("data is too big")
	}

	rtpTs := e.initialTs + uint32((at.Timestamp).Seconds()*e.clockRate)

	payload := []byte{0x00, 0x10}

	// 13 bits payload size
	// 3 bits AU-Index(-delta)
	header := make([]byte, 2)
	binary.BigEndian.PutUint16(header, uint16(len(at.AU))<<3)
	payload = append(payload, header...)

	payload = append(payload, at.AU...)

	rpkt := rtp.Packet{
		Header: rtp.Header{
			Version:        rtpVersion,
			PayloadType:    e.payloadType,
			SequenceNumber: e.sequenceNumber,
			Timestamp:      rtpTs,
			SSRC:           e.ssrc,
		},
		Payload: payload,
	}
	e.sequenceNumber++
	rpkt.Header.Marker = true

	frame, err := rpkt.Marshal()
	if err != nil {
		return nil, err
	}

	return frame, nil
}
