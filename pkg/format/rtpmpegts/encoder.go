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
	if e.PayloadMaxSize%MPEGTSPacketSize != 0 {
		return fmt.Errorf("PayloadMaxSize %d is not a multiple of %d", e.PayloadMaxSize, MPEGTSPacketSize)
	}

	e.sequenceNumber = *e.InitialSequenceNumber
	return nil
}

// Encode encodes MPEG-TS packets into RTP packets.
func (e *Encoder) Encode(tsData []byte) ([]*rtp.Packet, error) {
	if len(tsData) == 0 {
		return nil, fmt.Errorf("tsData is empty")
	}

	dataLen := len(tsData)
	if dataLen%MPEGTSPacketSize != 0 {
		return nil, fmt.Errorf("tsData length %d is not a multiple of %d", dataLen, MPEGTSPacketSize)
	}

	var rets []*rtp.Packet
	for i := 0; i < dataLen; i += e.PayloadMaxSize {
		end := min(i+e.PayloadMaxSize, dataLen)
		chunk := tsData[i:end]
		pkt := &rtp.Packet{
			Header: rtp.Header{
				Version:        rtpVersion,
				PayloadType:    payloadType,
				SequenceNumber: e.sequenceNumber,
				SSRC:           *e.SSRC,
			},
			Payload: chunk,
		}
		rets = append(rets, pkt)
		e.sequenceNumber++
	}

	return rets, nil
}
