package rtph265

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

	// initial timestamp of packets (optional).
	// It defaults to a random value.
	InitialTimestamp *uint32

	// maximum size of packet payloads (optional).
	// It defaults to 1460.
	PayloadMaxSize int

	// indicates that NALUs have an additional field that specifies the decoding order.
	MaxDONDiff int

	sequenceNumber uint16
	timeEncoder    *rtptime.Encoder
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
	e.timeEncoder = rtptime.NewEncoder(rtpClockRate, *e.InitialTimestamp)
	return nil
}

// Encode encodes NALUs into RTP/H265 packets.
func (e *Encoder) Encode(nalus [][]byte, pts time.Duration) ([]*rtp.Packet, error) {
	var rets []*rtp.Packet
	var batch [][]byte

	// split NALUs into batches
	for _, nalu := range nalus {
		if e.lenAggregationUnit(batch, nalu) <= e.PayloadMaxSize {
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
		return e.writeFragmentationUnits(nalus[0], pts, marker)
	}

	return e.writeAggregationUnit(nalus, pts, marker)
}

func (e *Encoder) writeSingle(nalu []byte, pts time.Duration, marker bool) ([]*rtp.Packet, error) {
	pkt := &rtp.Packet{
		Header: rtp.Header{
			Version:        rtpVersion,
			PayloadType:    e.PayloadType,
			SequenceNumber: e.sequenceNumber,
			Timestamp:      e.timeEncoder.Encode(pts),
			SSRC:           *e.SSRC,
			Marker:         marker,
		},
		Payload: nalu,
	}

	e.sequenceNumber++

	return []*rtp.Packet{pkt}, nil
}

func (e *Encoder) writeFragmentationUnits(nalu []byte, pts time.Duration, marker bool) ([]*rtp.Packet, error) {
	avail := e.PayloadMaxSize - 3
	le := len(nalu) - 2
	n := le / avail
	lastPacketSize := le % avail
	if lastPacketSize > 0 {
		n++
	}

	ret := make([]*rtp.Packet, n)
	encPTS := e.timeEncoder.Encode(pts)

	head := nalu[:2]
	nalu = nalu[2:]

	for i := range ret {
		start := uint8(0)
		if i == 0 {
			start = 1
		}
		end := uint8(0)
		le := avail
		if i == (n - 1) {
			end = 1
			le = lastPacketSize
		}

		data := make([]byte, 3+le)
		data[0] = head[0]&0b10000001 | 49<<1
		data[1] = head[1]
		data[2] = (start << 7) | (end << 6) | (head[0]>>1)&0b111111
		copy(data[3:], nalu[:le])
		nalu = nalu[le:]

		ret[i] = &rtp.Packet{
			Header: rtp.Header{
				Version:        rtpVersion,
				PayloadType:    e.PayloadType,
				SequenceNumber: e.sequenceNumber,
				Timestamp:      encPTS,
				SSRC:           *e.SSRC,
				Marker:         (i == (n-1) && marker),
			},
			Payload: data,
		}

		e.sequenceNumber++
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

func (e *Encoder) writeAggregationUnit(nalus [][]byte, pts time.Duration, marker bool) ([]*rtp.Packet, error) {
	payload := make([]byte, e.lenAggregationUnit(nalus, nil))

	// header
	h := uint16(48) << 9
	payload[0] = byte(h >> 8)
	payload[1] = byte(h)
	pos := 2

	for _, nalu := range nalus {
		// size
		naluLen := len(nalu)
		payload[pos] = uint8(naluLen >> 8)
		payload[pos+1] = uint8(naluLen)
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
			Timestamp:      e.timeEncoder.Encode(pts),
			SSRC:           *e.SSRC,
			Marker:         marker,
		},
		Payload: payload,
	}

	e.sequenceNumber++

	return []*rtp.Packet{pkt}, nil
}
