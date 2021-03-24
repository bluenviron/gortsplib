package rtph264

import (
	"encoding/binary"
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

// Encode encodes NALUs into RTP/H264 packets.
// It always returns at least one RTP/H264 packet.
// RTP/H264 packets can be:
// * single
// * fragmented (FU-A)
// * aggregated (STAP-A)
func (e *Encoder) Encode(nts []*NALUAndTimestamp) ([][]byte, error) {
	var rets [][]byte
	var batch []*NALUAndTimestamp

	// split packets into batches
	for _, nt := range nts {
		// packets can be contained into a single aggregation unit
		if e.lenAggregated(batch, nt) < rtpPayloadMaxSize {
			// add packet to batch
			batch = append(batch, nt)

		} else {
			// write last batch
			if batch != nil {
				pkts, err := e.writeBatch(batch)
				if err != nil {
					return nil, err
				}
				rets = append(rets, pkts...)
			}

			// initialize new batch
			batch = []*NALUAndTimestamp{nt}
		}
	}

	// write last batch
	pkts, err := e.writeBatch(batch)
	if err != nil {
		return nil, err
	}
	rets = append(rets, pkts...)

	return rets, nil
}

func (e *Encoder) writeBatch(nts []*NALUAndTimestamp) ([][]byte, error) {
	if len(nts) == 1 {
		// the NALU fits into a single RTP packet, use a single payload
		if len(nts[0].NALU) < rtpPayloadMaxSize {
			return e.writeSingle(nts[0])
		}

		// split the NALU into multiple fragmentation payloads
		return e.writeFragmented(nts[0])
	}

	return e.writeAggregated(nts)
}

func (e *Encoder) writeSingle(nt *NALUAndTimestamp) ([][]byte, error) {
	rpkt := rtp.Packet{
		Header: rtp.Header{
			Version:        rtpVersion,
			PayloadType:    e.payloadType,
			SequenceNumber: e.sequenceNumber,
			Timestamp:      e.encodeTimestamp(nt.Timestamp),
			SSRC:           e.ssrc,
			Marker:         true,
		},
		Payload: nt.NALU,
	}
	e.sequenceNumber++

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

func (e *Encoder) lenAggregated(batch []*NALUAndTimestamp, additionalEl *NALUAndTimestamp) int {
	ret := 1 // header

	for _, bnt := range batch {
		ret += 2             // size
		ret += len(bnt.NALU) // unit
	}

	if additionalEl != nil {
		ret += 2                      // size
		ret += len(additionalEl.NALU) // unit
	}

	return ret
}

func (e *Encoder) writeAggregated(nts []*NALUAndTimestamp) ([][]byte, error) {
	payload := make([]byte, e.lenAggregated(nts, nil))
	payload[0] = uint8(codech264.NALUTypeStapA) // header
	pos := 1

	for _, nt := range nts {
		naluLen := len(nt.NALU)
		binary.BigEndian.PutUint16(payload[pos:], uint16(naluLen))
		pos += 2

		copy(payload[pos:], nt.NALU)
		pos += naluLen
	}

	rpkt := rtp.Packet{
		Header: rtp.Header{
			Version:        rtpVersion,
			PayloadType:    e.payloadType,
			SequenceNumber: e.sequenceNumber,
			Timestamp:      e.encodeTimestamp(nts[0].Timestamp),
			SSRC:           e.ssrc,
			Marker:         true,
		},
		Payload: payload,
	}
	e.sequenceNumber++

	frame, err := rpkt.Marshal()
	if err != nil {
		return nil, err
	}

	return [][]byte{frame}, nil
}
