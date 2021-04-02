package rtph264

import (
	"encoding/binary"
	"fmt"
	"math/rand"
	"time"

	"github.com/pion/rtp"
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
// It can return:
// * a single packets
// * multiple fragmented packets (FU-A)
// * an aggregated packet (STAP-A)
func (e *Encoder) Encode(nts []*NALUAndTimestamp) ([][]byte, error) {
	var rets [][]byte
	var batch []*NALUAndTimestamp

	// split NALUs into batches
	for _, nt := range nts {
		if len(batch) > 0 && batch[0].Timestamp != nt.Timestamp {
			return nil, fmt.Errorf("encoding NALUs with different timestamps is unimplemented")
		}

		if e.lenAggregated(batch, nt) <= rtpPayloadMaxSize {
			// add to existing batch
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
		// the NALU fits into a single RTP packet
		if len(nts[0].NALU) < rtpPayloadMaxSize {
			return e.writeSingle(nts[0])
		}

		// split the NALU into multiple fragmentation packet
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
	packetCount := (len(nalu) - 1) / (rtpPayloadMaxSize - 2)
	lastPacketSize := (len(nalu) - 1) % (rtpPayloadMaxSize - 2)
	if lastPacketSize > 0 {
		packetCount++
	}

	ret := make([][]byte, packetCount)
	ts := e.encodeTimestamp(nt.Timestamp)

	nri := (nalu[0] >> 5) & 0x03
	typ := nalu[0] & 0x1F
	nalu = nalu[1:] // remove header

	for i := range ret {
		indicator := (nri << 5) | uint8(NALUTypeFuA)

		start := uint8(0)
		if i == 0 {
			start = 1
		}
		end := uint8(0)
		le := rtpPayloadMaxSize - 2
		if i == (packetCount - 1) {
			end = 1
			le = lastPacketSize
		}
		header := (start << 7) | (end << 6) | typ

		data := make([]byte, 2+le)
		data[0] = indicator
		data[1] = header
		copy(data[2:], nalu[:le])
		nalu = nalu[le:]

		rpkt := rtp.Packet{
			Header: rtp.Header{
				Version:        rtpVersion,
				PayloadType:    e.payloadType,
				SequenceNumber: e.sequenceNumber,
				Timestamp:      ts,
				SSRC:           e.ssrc,
				Marker:         (i == (packetCount - 1)),
			},
			Payload: data,
		}
		e.sequenceNumber++

		frame, err := rpkt.Marshal()
		if err != nil {
			return nil, err
		}

		ret[i] = frame
	}

	return ret, nil
}

func (e *Encoder) lenAggregated(nts []*NALUAndTimestamp, additionalEl *NALUAndTimestamp) int {
	ret := 1 // header

	for _, bnt := range nts {
		ret += 2             // size
		ret += len(bnt.NALU) // nalu
	}

	if additionalEl != nil {
		ret += 2                      // size
		ret += len(additionalEl.NALU) // nalu
	}

	return ret
}

func (e *Encoder) writeAggregated(nts []*NALUAndTimestamp) ([][]byte, error) {
	payload := make([]byte, e.lenAggregated(nts, nil))

	// header
	payload[0] = uint8(NALUTypeStapA)
	pos := 1

	for _, nt := range nts {
		// size
		naluLen := len(nt.NALU)
		binary.BigEndian.PutUint16(payload[pos:], uint16(naluLen))
		pos += 2

		// nalu
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
