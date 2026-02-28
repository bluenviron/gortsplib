package rtpmpegts

import (
	"crypto/rand"
	"fmt"

	"github.com/pion/rtp"
)

const (
	rtpVersion            = 2
	defaultPayloadMaxSize = 1316 // (7 x 188)
	payloadType           = 33
)

func ptrOf[T any](v T) *T {
	return &v
}

func randUint32() (uint32, error) {
	var b [4]byte
	_, err := rand.Read(b[:])
	if err != nil {
		return 0, err
	}
	return uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3]), nil
}

// Encoder is a RTP/MPEG-TS encoder.
// Specification: RFC2250
type Encoder struct {
	// SSRC of packets (optional).
	// It defaults to a random value.
	SSRC *uint32

	// initial sequence number of packets (optional).
	// It defaults to a random value.
	InitialSequenceNumber *uint16

	// maximum size of packet payloads (optional).
	// It defaults to 1316. (multiple of 188)
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
		e.InitialSequenceNumber = ptrOf(uint16(v))
	}
	if e.PayloadMaxSize == 0 {
		e.PayloadMaxSize = defaultPayloadMaxSize
	}

	e.sequenceNumber = *e.InitialSequenceNumber
	return nil
}

// Encode encodes MPEG-TS packets into RTP packets.
func (e *Encoder) Encode(tsPackets [][]byte) ([]*rtp.Packet, error) {
	for _, pkt := range tsPackets {
		if len(pkt) != mpegtsPacketSize {
			return nil, fmt.Errorf("invalid MPEG-TS packet size: %d", len(pkt))
		}
	}

	tsPacketCount := len(tsPackets)
	maxTSPacketsPerRTPPacket := e.PayloadMaxSize / mpegtsPacketSize
	rtpPacketCount := tsPacketCount / maxTSPacketsPerRTPPacket
	if tsPacketCount%maxTSPacketsPerRTPPacket != 0 {
		rtpPacketCount++
	}

	rets := make([]*rtp.Packet, rtpPacketCount)

	for i := range rtpPacketCount {
		if i == (rtpPacketCount - 1) {
			tsPacketCount = len(tsPackets)
		} else {
			tsPacketCount = maxTSPacketsPerRTPPacket
		}

		payload := make([]byte, tsPacketCount*mpegtsPacketSize)
		n := 0

		for range tsPacketCount {
			n += copy(payload[n:], tsPackets[0])
			tsPackets = tsPackets[1:]
		}

		rets[i] = &rtp.Packet{
			Header: rtp.Header{
				Version:        rtpVersion,
				PayloadType:    payloadType,
				SequenceNumber: e.sequenceNumber,
				SSRC:           *e.SSRC,
			},
			Payload: payload,
		}
		e.sequenceNumber++
	}

	return rets, nil
}
