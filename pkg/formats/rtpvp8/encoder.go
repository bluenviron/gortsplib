package rtpvp8

import (
	"crypto/rand"
	"fmt"
	"time"

	"github.com/pion/rtp"
	"github.com/pion/rtp/codecs"

	"github.com/bluenviron/gortsplib/v3/pkg/rtptime"
)

const (
	rtpVersion = 2
)

func randUint32() uint32 {
	var b [4]byte
	rand.Read(b[:])
	return uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3])
}

// Encoder is a RTP/VP8 encoder.
// Specification: https://datatracker.ietf.org/doc/html/rfc7741
type Encoder struct {
	// payload type of packets.
	PayloadType uint8

	// SSRC of packets (optional).
	// It defaults to a random value.
	SSRC *uint32

	// initial sequence number of packets (optional).
	// It defaults to a random value.
	InitialSequenceNumber *uint16

	// initial timestamp of packets (optional).
	// It defaults to a random value.
	InitialTimestamp *uint32

	// maximum size of packet payloads (optional).
	// It defaults to 1460.
	PayloadMaxSize int

	sequenceNumber uint16
	timeEncoder    *rtptime.Encoder
	vp             codecs.VP8Payloader
}

// Init initializes the encoder.
func (e *Encoder) Init() error {
	if e.SSRC == nil {
		v := randUint32()
		e.SSRC = &v
	}
	if e.InitialSequenceNumber == nil {
		v := uint16(randUint32())
		e.InitialSequenceNumber = &v
	}
	if e.InitialTimestamp == nil {
		v := randUint32()
		e.InitialTimestamp = &v
	}
	if e.PayloadMaxSize == 0 {
		e.PayloadMaxSize = 1460 // 1500 (UDP MTU) - 20 (IP header) - 8 (UDP header) - 12 (RTP header)
	}

	e.sequenceNumber = *e.InitialSequenceNumber
	e.timeEncoder = rtptime.NewEncoder(rtpClockRate, *e.InitialTimestamp)
	return nil
}

// Encode encodes a VP8 frame into RTP/VP8 packets.
func (e *Encoder) Encode(frame []byte, pts time.Duration) ([]*rtp.Packet, error) {
	payloads := e.vp.Payload(uint16(e.PayloadMaxSize), frame)
	if payloads == nil {
		return nil, fmt.Errorf("payloader failed")
	}

	plen := len(payloads)
	ret := make([]*rtp.Packet, plen)

	for i, payload := range payloads {
		ret[i] = &rtp.Packet{
			Header: rtp.Header{
				Version:        rtpVersion,
				PayloadType:    e.PayloadType,
				SequenceNumber: e.sequenceNumber,
				Timestamp:      e.timeEncoder.Encode(pts),
				SSRC:           *e.SSRC,
				Marker:         i == (plen - 1),
			},
			Payload: payload,
		}

		e.sequenceNumber++
	}

	return ret, nil
}
