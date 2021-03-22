package rtph264

import (
	"math/rand"
	"time"

	"github.com/pion/rtp"

	"github.com/aler9/gortsplib/pkg/codech264"
)

const (
	rtpVersion        = 0x02
	rtpPayloadMaxSize = 1460  // 1500 (mtu) - 20 (ip header) - 8 (udp header) - 12 (rtp header)
	rtpClockRate      = 90000 // h264 always uses 90khz
)

// Encoder is a RTP/H264 encoder.
type Encoder struct {
	payloadType    uint8
	sequenceNumber uint16
	ssrc           uint32
	initialTs      uint32
}

// NewEncoder allocates an Encoder.
func NewEncoder(payloadType uint8,
	sequenceNumber *uint16,
	ssrc *uint32,
	initialTs *uint32) *Encoder {
	return &Encoder{
		payloadType: payloadType,
		sequenceNumber: func() uint16 {
			if sequenceNumber != nil {
				return *sequenceNumber
			}
			return uint16(rand.Uint32())
		}(),
		ssrc: func() uint32 {
			if ssrc != nil {
				return *ssrc
			}
			return rand.Uint32()
		}(),
		initialTs: func() uint32 {
			if initialTs != nil {
				return *initialTs
			}
			return rand.Uint32()
		}(),
	}
}

func (e *Encoder) encodeTimestamp(ts time.Duration) uint32 {
	return e.initialTs + uint32(ts.Seconds()*rtpClockRate)
}

// Encode encodes a NALU into RTP/H264 packets.
// It always returns at least one RTP/H264 packet.
func (e *Encoder) Encode(nt *NALUAndTimestamp) ([][]byte, error) {
	// if the NALU fits into a single RTP packet, use a single payload
	if len(nt.NALU) < rtpPayloadMaxSize {
		return e.writeSingle(nt)
	}

	// otherwise, split the NALU into multiple fragmentation payloads
	return e.writeFragmented(nt)
}

func (e *Encoder) writeSingle(nt *NALUAndTimestamp) ([][]byte, error) {
	rpkt := rtp.Packet{
		Header: rtp.Header{
			Version:        rtpVersion,
			PayloadType:    e.payloadType,
			SequenceNumber: e.sequenceNumber,
			Timestamp:      e.encodeTimestamp(nt.Timestamp),
			SSRC:           e.ssrc,
		},
		Payload: nt.NALU,
	}
	e.sequenceNumber++

	rpkt.Header.Marker = true

	frame, err := rpkt.Marshal()
	if err != nil {
		return nil, err
	}

	return [][]byte{frame}, nil
}

func (e *Encoder) writeFragmented(nt *NALUAndTimestamp) ([][]byte, error) {
	nalu := nt.NALU

	// use only FU-A, not FU-B, since we always use non-interleaved mode
	// (packetization-mode=1)
	frameCount := (len(nalu) - 1) / (rtpPayloadMaxSize - 2)
	lastFrameSize := (len(nalu) - 1) % (rtpPayloadMaxSize - 2)
	if lastFrameSize > 0 {
		frameCount++
	}
	ret := make([][]byte, frameCount)

	nri := (nalu[0] >> 5) & 0x03
	typ := nalu[0] & 0x1F
	nalu = nalu[1:] // remove header

	ts := e.encodeTimestamp(nt.Timestamp)

	for i := 0; i < frameCount; i++ {
		indicator := (nri << 5) | uint8(codech264.NALUTypeFuA)

		start := uint8(0)
		if i == 0 {
			start = 1
		}
		end := uint8(0)
		le := rtpPayloadMaxSize - 2
		if i == (frameCount - 1) {
			end = 1
			le = lastFrameSize
		}
		header := (start << 7) | (end << 6) | typ

		data := append([]byte{indicator, header}, nalu[:le]...)
		nalu = nalu[le:]

		rpkt := rtp.Packet{
			Header: rtp.Header{
				Version:        rtpVersion,
				PayloadType:    e.payloadType,
				SequenceNumber: e.sequenceNumber,
				Timestamp:      ts,
				SSRC:           e.ssrc,
			},
			Payload: data,
		}
		e.sequenceNumber++

		if i == (frameCount - 1) {
			rpkt.Header.Marker = true
		}

		frame, err := rpkt.Marshal()
		if err != nil {
			return nil, err
		}

		ret[i] = frame
	}

	return ret, nil
}
