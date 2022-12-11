package rtpvp9

import (
	"crypto/rand"
	"fmt"
	"time"

	"github.com/pion/rtp"
	"github.com/pion/rtp/codecs"
)

const (
	rtpVersion = 2
)

func randUint32() uint32 {
	var b [4]byte
	rand.Read(b[:])
	return uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3])
}

// Encoder is a RTP/VP9 encoder.
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

	// initial picture ID of frames (optional).
	// It defaults to a random value.
	InitialPictureID *uint16

	sequenceNumber uint16
	vp             codecs.VP9Payloader
}

// Init initializes the encoder.
func (e *Encoder) Init() {
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
	if e.InitialPictureID == nil {
		v := uint16(randUint32())
		e.InitialPictureID = &v
	}

	e.sequenceNumber = *e.InitialSequenceNumber

	e.vp.InitialPictureIDFn = func() uint16 {
		return *e.InitialPictureID
	}
}

func (e *Encoder) encodeTimestamp(ts time.Duration) uint32 {
	return *e.InitialTimestamp + uint32(ts.Seconds()*rtpClockRate)
}

// Encode encodes a VP9 frame into RTP/VP9 packets.
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
				Timestamp:      e.encodeTimestamp(pts),
				SSRC:           *e.SSRC,
				Marker:         i == (plen - 1),
			},
			Payload: payload,
		}

		e.sequenceNumber++
	}

	return ret, nil
}
