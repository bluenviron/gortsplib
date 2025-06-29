package rtph265

import (
	"crypto/rand"
	"fmt"

	"github.com/pion/rtp"
)

const (
	rtpVersion            = 2
	defaultPayloadMaxSize = 1450 // 1500 (UDP MTU) - 20 (IP header) - 8 (UDP header) - 12 (RTP header) - 10 (SRTP overhead)
)

func randUint32() (uint32, error) {
	var b [4]byte
	_, err := rand.Read(b[:])
	if err != nil {
		return 0, err
	}
	return uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3]), nil
}

func packetCount(avail, le int) int {
	n := le / avail
	if (le % avail) != 0 {
		n++
	}
	return n
}

// Encoder is a RTP/H265 encoder.
// Specification: https://datatracker.ietf.org/doc/html/rfc7798
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
	// It defaults to 1450.
	PayloadMaxSize int

	// indicates that NALUs have an additional field that specifies the decoding order.
	MaxDONDiff int

	sequenceNumber uint16
}

// Init initializes the encoder.
func (e *Encoder) Init() error {
	if e.MaxDONDiff != 0 {
		return fmt.Errorf("MaxDONDiff != 0 is not supported (yet)")
	}

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

// Encode encodes an access unit into RTP/H265 packets.
func (e *Encoder) Encode(au [][]byte) ([]*rtp.Packet, error) {
	var rets []*rtp.Packet
	var batch [][]byte

	// split NALUs into batches
	for _, nalu := range au {
		if e.lenAggregationUnit(batch, nalu) <= e.PayloadMaxSize {
			// add to existing batch
			batch = append(batch, nalu)
		} else {
			// write batch
			if batch != nil {
				pkts, err := e.writeBatch(batch, false)
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
	// marker is used to indicate that the entire access unit has been sent
	pkts, err := e.writeBatch(batch, true)
	if err != nil {
		return nil, err
	}
	rets = append(rets, pkts...)

	return rets, nil
}

func (e *Encoder) writeBatch(nalus [][]byte, marker bool) ([]*rtp.Packet, error) {
	if len(nalus) == 1 {
		// the NALU fits into a single RTP packet
		if len(nalus[0]) < e.PayloadMaxSize {
			return e.writeSingle(nalus[0], marker)
		}

		// split the NALU into multiple fragmentation packet
		return e.writeFragmentationUnits(nalus[0], marker)
	}

	return e.writeAggregationUnit(nalus, marker)
}

func (e *Encoder) writeSingle(nalu []byte, marker bool) ([]*rtp.Packet, error) {
	pkt := &rtp.Packet{
		Header: rtp.Header{
			Version:        rtpVersion,
			PayloadType:    e.PayloadType,
			SequenceNumber: e.sequenceNumber,
			SSRC:           *e.SSRC,
			Marker:         marker,
		},
		Payload: nalu,
	}

	e.sequenceNumber++

	return []*rtp.Packet{pkt}, nil
}

func (e *Encoder) writeFragmentationUnits(nalu []byte, marker bool) ([]*rtp.Packet, error) {
	avail := e.PayloadMaxSize - 3
	le := len(nalu) - 2
	packetCount := packetCount(avail, le)

	ret := make([]*rtp.Packet, packetCount)

	head := nalu[:2]
	nalu = nalu[2:]
	le = avail
	start := uint8(1)
	end := uint8(0)

	for i := range ret {
		if i == (packetCount - 1) {
			le = len(nalu)
			end = 1
		}

		data := make([]byte, 3+le)
		data[0] = head[0]&0b10000001 | 49<<1
		data[1] = head[1]
		data[2] = (start << 7) | (end << 6) | (head[0]>>1)&0b111111
		copy(data[3:], nalu)
		nalu = nalu[le:]

		ret[i] = &rtp.Packet{
			Header: rtp.Header{
				Version:        rtpVersion,
				PayloadType:    e.PayloadType,
				SequenceNumber: e.sequenceNumber,
				SSRC:           *e.SSRC,
				Marker:         (i == (packetCount-1) && marker),
			},
			Payload: data,
		}

		e.sequenceNumber++
		start = 0
	}

	return ret, nil
}

func (e *Encoder) lenAggregationUnit(nalus [][]byte, addNALU []byte) int {
	ret := 2 // header

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

func (e *Encoder) writeAggregationUnit(nalus [][]byte, marker bool) ([]*rtp.Packet, error) {
	payload := make([]byte, e.lenAggregationUnit(nalus, nil))

	layerID := byte(0xFF)
	temporalID := byte(0xFF)
	pos := 2

	for _, nalu := range nalus {
		if len(nalu) < 2 {
			return nil, fmt.Errorf("invalid NALU")
		}

		// select lowest layerID & temporalID
		nalLayerID := ((nalu[0] & 0x01) << 5) | ((nalu[1] >> 3) & 0x1F)
		nalTemporalID := nalu[1] & 0x07
		if nalLayerID < layerID {
			layerID = nalLayerID
		}
		if nalTemporalID < temporalID {
			temporalID = nalTemporalID
		}

		// size
		naluLen := len(nalu)
		payload[pos] = uint8(naluLen >> 8)
		payload[pos+1] = uint8(naluLen)
		pos += 2

		// nalu
		copy(payload[pos:], nalu)
		pos += naluLen
	}

	// header
	payload[0] = (48 << 1) | (layerID & 0x20)
	payload[1] = ((layerID & 0x1F) << 3) | (temporalID & 0x07)

	pkt := &rtp.Packet{
		Header: rtp.Header{
			Version:        rtpVersion,
			PayloadType:    e.PayloadType,
			SequenceNumber: e.sequenceNumber,
			SSRC:           *e.SSRC,
			Marker:         marker,
		},
		Payload: payload,
	}

	e.sequenceNumber++

	return []*rtp.Packet{pkt}, nil
}
