package rtpmpeg4audiolatm

import (
	"crypto/rand"
	"fmt"
	"time"

	"github.com/bluenviron/mediacommon/pkg/codecs/mpeg4audio"
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

// Encoder is a RTP/MPEG4-audio encoder.
// Specification: https://datatracker.ietf.org/doc/html/rfc6416#section-7.3
type Encoder struct {
	// payload type of packets.
	PayloadType uint8

	// StreamMuxConfig.
	Config *mpeg4audio.StreamMuxConfig

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
func (e *Encoder) Init() error {
	if e.Config == nil || len(e.Config.Programs) != 1 || len(e.Config.Programs[0].Layers) != 1 {
		return fmt.Errorf("unsupported StreamMuxConfig")
	}

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
	e.timeEncoder = rtptime.NewEncoder(e.Config.Programs[0].Layers[0].AudioSpecificConfig.SampleRate, *e.InitialTimestamp)
	return nil
}

func payloadLengthInfoLen(auLen int) int {
	return auLen/255 + 1
}

func payloadLengthInfo(plil int, auLen int, buf []byte) {
	for i := 0; i < plil; i++ {
		buf[i] = 255
	}
	buf[plil-1] = byte(auLen % 255)
}

func (e *Encoder) packetCount(auLen int, plil int) int {
	totalLen := plil + auLen
	packetCount := totalLen / e.PayloadMaxSize
	lastPacketSize := totalLen % e.PayloadMaxSize
	if lastPacketSize > 0 {
		packetCount++
	}
	return packetCount
}

// Encode encodes AUs into RTP packets.
func (e *Encoder) Encode(au []byte, pts time.Duration) ([]*rtp.Packet, error) {
	auLen := len(au)
	plil := payloadLengthInfoLen(auLen)
	packetCount := e.packetCount(auLen, plil)

	avail := e.PayloadMaxSize - plil
	ret := make([]*rtp.Packet, packetCount)
	encPTS := e.timeEncoder.Encode(pts)

	for i := range ret {
		var final bool
		var l int

		if len(au) < avail {
			l = len(au)
			final = true
		} else {
			l = avail
			final = false
		}

		var payload []byte

		if i == 0 {
			payload = make([]byte, plil+l)
			payloadLengthInfo(plil, auLen, payload)
			copy(payload[plil:], au[:l])
		} else {
			payload = au[:l]
		}

		ret[i] = &rtp.Packet{
			Header: rtp.Header{
				Version:        rtpVersion,
				PayloadType:    e.PayloadType,
				SequenceNumber: e.sequenceNumber,
				Timestamp:      encPTS,
				SSRC:           *e.SSRC,
				Marker:         final,
			},
			Payload: payload,
		}

		e.sequenceNumber++

		if final {
			break
		}

		au = au[l:]
		avail = e.PayloadMaxSize
	}

	return ret, nil
}
