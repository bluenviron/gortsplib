package rtpklv

import (
	"crypto/rand"

	"github.com/pion/rtp"
)

const (
	rtpVersion            = 2
	defaultPayloadMaxSize = 1450 // 1500 (UDP MTU) - 20 (IP header) - 8 (UDP header) - 12 (RTP header) - 10 (SRTP overhead)
)

func randUint32() (uint32, error) {
	var b [4]byte
	_, err := rand.Read(b[:])
	if err != nil {
		return 0, err
	}
	return uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3]), nil
}

// Encoder is a RTP/KLV encoder.
// Specification: RFC6597
type Encoder struct {
	// payload type of packets.
	PayloadType uint8

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
		v2 := uint16(v)
		e.InitialSequenceNumber = &v2
	}
	if e.PayloadMaxSize == 0 {
		e.PayloadMaxSize = defaultPayloadMaxSize
	}

	e.sequenceNumber = *e.InitialSequenceNumber
	return nil
}

// Encode encodes a KLV unit into RTP packets.
func (e *Encoder) Encode(unit []byte) ([]*rtp.Packet, error) {
	var packets []*rtp.Packet

	if len(unit) <= e.PayloadMaxSize {
		pkt := &rtp.Packet{
			Header: rtp.Header{
				Version:        rtpVersion,
				PayloadType:    e.PayloadType,
				SequenceNumber: e.sequenceNumber,
				SSRC:           *e.SSRC,
				Marker:         true, // Single packet, so this is the last (and only) packet
			},
			Payload: unit,
		}
		e.sequenceNumber++
		return []*rtp.Packet{pkt}, nil
	}

	// KLV unit needs to be fragmented across multiple packets
	offset := 0
	for offset < len(unit) {
		// Calculate payload size for this packet
		payloadSize := e.PayloadMaxSize
		if offset+payloadSize > len(unit) {
			payloadSize = len(unit) - offset
		}

		// Determine if this is the last packet
		isLast := (offset + payloadSize) >= len(unit)

		// Create the packet
		pkt := &rtp.Packet{
			Header: rtp.Header{
				Version:        rtpVersion,
				PayloadType:    e.PayloadType,
				SequenceNumber: e.sequenceNumber,
				SSRC:           *e.SSRC,
				Marker:         isLast, // Set marker bit only on the last packet
			},
			Payload: unit[offset : offset+payloadSize],
		}

		packets = append(packets, pkt)
		e.sequenceNumber++
		offset += payloadSize
	}

	return packets, nil
}
