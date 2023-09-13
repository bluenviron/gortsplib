package rtpmpeg4audiolatm

import (
	"crypto/rand"

	"github.com/pion/rtp"
)

const (
	rtpVersion            = 2
	defaultPayloadMaxSize = 1460 // 1500 (UDP MTU) - 20 (IP header) - 8 (UDP header) - 12 (RTP header)
)

func randUint32() (uint32, error) {
	var b [4]byte
	_, err := rand.Read(b[:])
	if err != nil {
		return 0, err
	}
	return uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3]), nil
}

// Encoder is a RTP/MPEG4-audio encoder.
// Specification: https://datatracker.ietf.org/doc/html/rfc6416#section-7.3
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
	// It defaults to 1460.
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

func (e *Encoder) packetCount(auLen int, plil int) int {
	totalLen := plil + auLen
	n := totalLen / e.PayloadMaxSize
	if (totalLen % e.PayloadMaxSize) != 0 {
		n++
	}
	return n
}

// Encode encodes an access unit into RTP packets.
func (e *Encoder) Encode(au []byte) ([]*rtp.Packet, error) {
	auLen := len(au)
	plil := payloadLengthInfoEncodeSize(auLen)
	packetCount := e.packetCount(auLen, plil)

	ret := make([]*rtp.Packet, packetCount)
	le := e.PayloadMaxSize - plil

	for i := range ret {
		if i == (packetCount - 1) {
			le = len(au)
		}

		var payload []byte

		if i == 0 {
			payload = make([]byte, plil+le)
			payloadLengthInfoEncode(plil, auLen, payload)
			copy(payload[plil:], au[:le])
			au = au[le:]
			le = e.PayloadMaxSize
		} else {
			payload = au[:le]
			au = au[le:]
		}

		ret[i] = &rtp.Packet{
			Header: rtp.Header{
				Version:        rtpVersion,
				PayloadType:    e.PayloadType,
				SequenceNumber: e.sequenceNumber,
				SSRC:           *e.SSRC,
				Marker:         (i == packetCount-1),
			},
			Payload: payload,
		}

		e.sequenceNumber++
	}

	return ret, nil
}
