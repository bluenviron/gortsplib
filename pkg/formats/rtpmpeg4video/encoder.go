package rtpmpeg4video

import (
	"crypto/rand"
	"time"

	"github.com/pion/rtp"

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

// Encoder is a RTP/MPEG-4 Video encoder.
// Specification: https://datatracker.ietf.org/doc/html/rfc6416
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
	e.timeEncoder = rtptime.NewEncoder(90000, *e.InitialTimestamp)
}

// Encode encodes a frame into RTP packets.
func (e *Encoder) Encode(frame []byte, pts time.Duration) ([]*rtp.Packet, error) {
	availPerPacket := e.PayloadMaxSize
	le := len(frame)
	packetCount := le / availPerPacket
	lastPacketSize := le % availPerPacket
	if lastPacketSize > 0 {
		packetCount++
	}

	pos := 0
	ret := make([]*rtp.Packet, packetCount)
	encPTS := e.timeEncoder.Encode(pts)

	for i := range ret {
		var le int
		if i != (packetCount - 1) {
			le = availPerPacket
		} else {
			le = lastPacketSize
		}

		payload := make([]byte, le)
		pos += copy(payload, frame[pos:])

		ret[i] = &rtp.Packet{
			Header: rtp.Header{
				Version:        rtpVersion,
				PayloadType:    e.PayloadType,
				SequenceNumber: e.sequenceNumber,
				Timestamp:      encPTS,
				SSRC:           *e.SSRC,
				Marker:         (i == len(ret)-1),
			},
			Payload: payload,
		}

		e.sequenceNumber++
	}

	return ret, nil
}
