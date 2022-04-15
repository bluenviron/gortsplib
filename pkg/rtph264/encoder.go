package rtph264

import (
	"crypto/rand"
	"encoding/binary"
	"time"

	"github.com/pion/rtp"
)

func randUint32() uint32 {
	var b [4]byte
	rand.Read(b[:])
	return uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3])
}

// Encoder is a RTP/H264 encoder.
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
	return *e.InitialTimestamp + uint32(ts.Seconds()*rtpClockRate)
}

// Encode encodes NALUs into RTP/H264 packets.
func (e *Encoder) Encode(nalus [][]byte, pts time.Duration) ([]*rtp.Packet, error) {
	var rets []*rtp.Packet
	var batch [][]byte

	// split NALUs into batches
	for _, nalu := range nalus {
		if e.lenAggregated(batch, nalu) <= e.PayloadMaxSize {
			// add to existing batch
			batch = append(batch, nalu)
		} else {
			// write batch
			if batch != nil {
				pkts, err := e.writeBatch(batch, pts, false)
				if err != nil {
					return nil, err
				}
				rets = append(rets, pkts...)
			}

			// initialize new batch
			batch = [][]byte{nalu}
		}
	}

	// write final batch
	// marker is used to indicate when all NALUs with same PTS have been sent
	pkts, err := e.writeBatch(batch, pts, true)
	if err != nil {
		return nil, err
	}
	rets = append(rets, pkts...)

	return rets, nil
}

func (e *Encoder) writeBatch(nalus [][]byte, pts time.Duration, marker bool) ([]*rtp.Packet, error) {
	if len(nalus) == 1 {
		// the NALU fits into a single RTP packet
		if len(nalus[0]) < e.PayloadMaxSize {
			return e.writeSingle(nalus[0], pts, marker)
		}

		// split the NALU into multiple fragmentation packet
		return e.writeFragmented(nalus[0], pts, marker)
	}

	return e.writeAggregated(nalus, pts, marker)
}

func (e *Encoder) writeSingle(nalu []byte, pts time.Duration, marker bool) ([]*rtp.Packet, error) {
	pkt := &rtp.Packet{
		Header: rtp.Header{
			Version:        rtpVersion,
			PayloadType:    e.PayloadType,
			SequenceNumber: e.sequenceNumber,
			Timestamp:      e.encodeTimestamp(pts),
			SSRC:           *e.SSRC,
			Marker:         marker,
		},
		Payload: nalu,
	}

	e.sequenceNumber++

	return []*rtp.Packet{pkt}, nil
}

func (e *Encoder) writeFragmented(nalu []byte, pts time.Duration, marker bool) ([]*rtp.Packet, error) {
	// use only FU-A, not FU-B, since we always use non-interleaved mode
	// (packetization-mode=1)
	packetCount := (len(nalu) - 1) / (e.PayloadMaxSize - 2)
	lastPacketSize := (len(nalu) - 1) % (e.PayloadMaxSize - 2)
	if lastPacketSize > 0 {
		packetCount++
	}

	ret := make([]*rtp.Packet, packetCount)
	encPTS := e.encodeTimestamp(pts)

	nri := (nalu[0] >> 5) & 0x03
	typ := nalu[0] & 0x1F
	nalu = nalu[1:] // remove header

	for i := range ret {
		indicator := (nri << 5) | uint8(naluTypeFUA)

		start := uint8(0)
		if i == 0 {
			start = 1
		}
		end := uint8(0)
		le := e.PayloadMaxSize - 2
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

		ret[i] = &rtp.Packet{
			Header: rtp.Header{
				Version:        rtpVersion,
				PayloadType:    e.PayloadType,
				SequenceNumber: e.sequenceNumber,
				Timestamp:      encPTS,
				SSRC:           *e.SSRC,
				Marker:         (i == (packetCount-1) && marker),
			},
			Payload: data,
		}

		e.sequenceNumber++
	}

	return ret, nil
}

func (e *Encoder) lenAggregated(nalus [][]byte, addNALU []byte) int {
	ret := 1 // header

	for _, nalu := range nalus {
		ret += 2         // size
		ret += len(nalu) // nalu
	}

	if addNALU != nil {
		ret += 2            // size
		ret += len(addNALU) // nalu
	}

	return ret
}

func (e *Encoder) writeAggregated(nalus [][]byte, pts time.Duration, marker bool) ([]*rtp.Packet, error) {
	payload := make([]byte, e.lenAggregated(nalus, nil))

	// header
	payload[0] = uint8(naluTypeSTAPA)
	pos := 1

	for _, nalu := range nalus {
		// size
		naluLen := len(nalu)
		binary.BigEndian.PutUint16(payload[pos:], uint16(naluLen))
		pos += 2

		// nalu
		copy(payload[pos:], nalu)
		pos += naluLen
	}

	pkt := &rtp.Packet{
		Header: rtp.Header{
			Version:        rtpVersion,
			PayloadType:    e.PayloadType,
			SequenceNumber: e.sequenceNumber,
			Timestamp:      e.encodeTimestamp(pts),
			SSRC:           *e.SSRC,
			Marker:         marker,
		},
		Payload: payload,
	}

	e.sequenceNumber++

	return []*rtp.Packet{pkt}, nil
}
