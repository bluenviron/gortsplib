package rtplpcm

import (
	"crypto/rand"
	"fmt"
	"time"

	"github.com/pion/rtp"

	"github.com/bluenviron/gortsplib/v3/pkg/rtptime"
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

// Encoder is a RTP/LPCM encoder.
// Specification: https://datatracker.ietf.org/doc/html/rfc3190
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

	BitDepth     int
	SampleRate   int
	ChannelCount int

	sequenceNumber uint16
	timeEncoder    *rtptime.Encoder
	sampleSize     int
	maxPayloadSize int
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
	if e.InitialTimestamp == nil {
		v, err := randUint32()
		if err != nil {
			return err
		}
		e.InitialTimestamp = &v
	}
	if e.PayloadMaxSize == 0 {
		e.PayloadMaxSize = defaultPayloadMaxSize
	}

	e.sequenceNumber = *e.InitialSequenceNumber
	e.timeEncoder = rtptime.NewEncoder(e.SampleRate, *e.InitialTimestamp)
	e.sampleSize = e.BitDepth * e.ChannelCount / 8
	e.maxPayloadSize = (e.PayloadMaxSize / e.sampleSize) * e.sampleSize
	return nil
}

func (e *Encoder) packetCount(slen int) int {
	n := (slen / e.maxPayloadSize)
	if (slen % e.maxPayloadSize) != 0 {
		n++
	}

	return n
}

// Encode encodes audio samples into RTP packets.
func (e *Encoder) Encode(samples []byte, pts time.Duration) ([]*rtp.Packet, error) {
	slen := len(samples)
	if (slen % e.sampleSize) != 0 {
		return nil, fmt.Errorf("invalid samples")
	}

	packetCount := e.packetCount(slen)
	ret := make([]*rtp.Packet, packetCount)
	i := 0
	pos := 0
	payloadSize := e.maxPayloadSize

	for {
		if payloadSize > len(samples[pos:]) {
			payloadSize = len(samples[pos:])
		}

		ret[i] = &rtp.Packet{
			Header: rtp.Header{
				Version:        rtpVersion,
				PayloadType:    e.PayloadType,
				SequenceNumber: e.sequenceNumber,
				Timestamp:      e.timeEncoder.Encode(pts),
				SSRC:           *e.SSRC,
				Marker:         false,
			},
			Payload: samples[pos : pos+payloadSize],
		}

		e.sequenceNumber++
		i++
		pos += payloadSize
		pts += time.Duration(payloadSize/e.sampleSize) * time.Second / time.Duration(e.SampleRate)

		if pos == slen {
			break
		}
	}

	return ret, nil
}
