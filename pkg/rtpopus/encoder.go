package rtpopus

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

// Encoder is a RTP/Opus encoder.
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

	// sample rate of packets.
	SampleRate int

	sequenceNumber uint16
	op             codecs.OpusPayloader
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

	e.sequenceNumber = *e.InitialSequenceNumber
}

func (e *Encoder) encodeTimestamp(ts time.Duration) uint32 {
	return *e.InitialTimestamp + uint32(ts.Seconds()*float64(e.SampleRate))
}

// Encode encodes an Opus packet into a RTP/Opus packet.
func (e *Encoder) Encode(op []byte, pts time.Duration) (*rtp.Packet, error) {
	if len(op) > e.PayloadMaxSize {
		return nil, fmt.Errorf("packet size exceeds maximum size")
	}

	pkt := &rtp.Packet{
		Header: rtp.Header{
			Version:        rtpVersion,
			PayloadType:    e.PayloadType,
			SequenceNumber: e.sequenceNumber,
			Timestamp:      e.encodeTimestamp(pts),
			SSRC:           *e.SSRC,
			Marker:         true,
		},
		Payload: e.op.Payload(0, op)[0],
	}

	e.sequenceNumber++

	return pkt, nil
}
