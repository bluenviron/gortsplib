// Package rtpaac contains a RTP/AAC decoder and encoder.
package rtpaac

import (
	"encoding/binary"
	"fmt"
	"math/rand"
	"time"

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
	started        time.Duration
}

// NewEncoder allocates an Encoder.
func NewEncoder(payloadType uint8, clockRate int) (*Encoder, error) {
	return &Encoder{
		payloadType:    payloadType,
		clockRate:      float64(clockRate),
		sequenceNumber: uint16(rand.Uint32()),
		ssrc:           rand.Uint32(),
		initialTs:      rand.Uint32(),
	}, nil
}

// Write encodes an AAC frame into RTP/AAC packets.
func (e *Encoder) Write(ts time.Duration, data []byte) ([][]byte, error) {
	if e.started == 0 {
		e.started = ts
	}

	if len(data) > rtpPayloadMaxSize {
		return nil, fmt.Errorf("data is too big")
	}

	rtpTs := e.initialTs + uint32((ts-e.started).Seconds()*e.clockRate)

	// 13 bits payload size
	// 3 bits AU-Index(-delta)
	header := make([]byte, 2)
	binary.BigEndian.PutUint16(header, (uint16(len(data))<<3)|0)

	payload := append([]byte{0x00, 0x10}, header...)
	payload = append(payload, data...)

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

	return [][]byte{frame}, nil
}
