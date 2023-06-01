package rtpmpeg2audio

import (
	"crypto/rand"
	"time"

	"github.com/bluenviron/mediacommon/pkg/codecs/mpeg2audio"
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

func lenAggregated(frames [][]byte, frame []byte) int {
	l := 4 + len(frame)
	for _, fr := range frames {
		l += len(fr)
	}
	return l
}

// Encoder is a RTP/MPEG-1/2 Audio encoder.
// Specification: https://datatracker.ietf.org/doc/html/rfc2250
type Encoder struct {
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
	return nil
}

// Encode encodes frames into RTP packets.
func (e *Encoder) Encode(frames [][]byte, pts time.Duration) ([]*rtp.Packet, error) {
	var rets []*rtp.Packet
	var batch [][]byte

	for _, frame := range frames {
		if lenAggregated(batch, frame) <= e.PayloadMaxSize {
			batch = append(batch, frame)
		} else {
			// write last batch
			if batch != nil {
				pkts, err := e.writeBatch(batch, pts)
				if err != nil {
					return nil, err
				}
				rets = append(rets, pkts...)

				for _, frame := range batch {
					var h mpeg2audio.FrameHeader
					err := h.Unmarshal(frame)
					if err != nil {
						return nil, err
					}

					pts += time.Duration(h.SampleCount()) * time.Second / time.Duration(h.SampleRate)
				}
			}

			// initialize new batch
			batch = [][]byte{frame}
		}
	}

	// write last batch
	pkts, err := e.writeBatch(batch, pts)
	if err != nil {
		return nil, err
	}
	rets = append(rets, pkts...)

	return rets, nil
}

func (e *Encoder) writeBatch(frames [][]byte, pts time.Duration) ([]*rtp.Packet, error) {
	if len(frames) != 1 || lenAggregated(frames, nil) < e.PayloadMaxSize {
		return e.writeAggregated(frames, pts)
	}

	return e.writeFragmented(frames[0], pts)
}

func (e *Encoder) writeFragmented(frame []byte, pts time.Duration) ([]*rtp.Packet, error) {
	avail := e.PayloadMaxSize - 4
	le := len(frame)
	packetCount := le / avail
	lastPacketSize := le % avail
	if lastPacketSize > 0 {
		packetCount++
	}

	pos := 0
	ret := make([]*rtp.Packet, packetCount)
	encPTS := e.timeEncoder.Encode(pts)

	for i := range ret {
		var le int
		if i != (packetCount - 1) {
			le = avail
		} else {
			le = lastPacketSize
		}

		payload := make([]byte, 4+le)
		payload[2] = byte(pos >> 8)
		payload[3] = byte(pos)

		pos += copy(payload[4:], frame[pos:])

		ret[i] = &rtp.Packet{
			Header: rtp.Header{
				Version:        rtpVersion,
				PayloadType:    14,
				SequenceNumber: e.sequenceNumber,
				Timestamp:      encPTS,
				SSRC:           *e.SSRC,
				Marker:         true,
			},
			Payload: payload,
		}

		e.sequenceNumber++
	}

	return ret, nil
}

func (e *Encoder) writeAggregated(frames [][]byte, pts time.Duration) ([]*rtp.Packet, error) {
	payload := make([]byte, lenAggregated(frames, nil))

	n := 4
	for _, frame := range frames {
		n += copy(payload[n:], frame)
	}

	pkt := &rtp.Packet{
		Header: rtp.Header{
			Version:        rtpVersion,
			PayloadType:    14,
			SequenceNumber: e.sequenceNumber,
			Timestamp:      e.timeEncoder.Encode(pts),
			SSRC:           *e.SSRC,
			Marker:         true,
		},
		Payload: payload,
	}

	e.sequenceNumber++

	return []*rtp.Packet{pkt}, nil
}
