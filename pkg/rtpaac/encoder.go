package rtpaac

import (
	"encoding/binary"
	"math/rand"
	"time"

	"github.com/pion/rtp"
)

const (
	rtpVersion        = 0x02
	rtpPayloadMaxSize = 1460 // 1500 (mtu) - 20 (ip header) - 8 (udp header) - 12 (rtp header)
)

// Encoder is a RTP/AAC encoder.
type Encoder struct {
	payloadType    uint8
	clockRate      float64
	sequenceNumber uint16
	ssrc           uint32
	initialTs      uint32
}

// NewEncoder allocates an Encoder.
func NewEncoder(payloadType uint8,
	clockRate int,
	sequenceNumber *uint16,
	ssrc *uint32,
	initialTs *uint32) *Encoder {
	return &Encoder{
		payloadType: payloadType,
		clockRate:   float64(clockRate),
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
	return e.initialTs + uint32(ts.Seconds()*e.clockRate)
}

// Encode encodes AUs into RTP/AAC packets.
func (e *Encoder) Encode(ats []*AUAndTimestamp) ([][]byte, error) {
	var rets [][]byte
	var batch []*AUAndTimestamp

	// split AUs into batches
	for _, at := range ats {
		if e.lenAggregated(batch, at) <= rtpPayloadMaxSize {
			// add to existing batch
			batch = append(batch, at)

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
			batch = []*AUAndTimestamp{at}
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

func (e *Encoder) writeBatch(ats []*AUAndTimestamp) ([][]byte, error) {
	if len(ats) == 1 {
		// the AU fits into a single RTP packet
		if len(ats[0].AU) < rtpPayloadMaxSize {
			return e.writeAggregated(ats)
		}

		// split the AU into multiple fragmentation packet
		return e.writeFragmented(ats[0])
	}

	return e.writeAggregated(ats)
}

func (e *Encoder) writeFragmented(at *AUAndTimestamp) ([][]byte, error) {
	au := at.AU

	packetCount := len(au) / (rtpPayloadMaxSize - 4)
	lastPacketSize := len(au) % (rtpPayloadMaxSize - 4)
	if lastPacketSize > 0 {
		packetCount++
	}

	ret := make([][]byte, packetCount)
	ts := e.encodeTimestamp(at.Timestamp)

	for i := range ret {
		le := rtpPayloadMaxSize - 4
		if i == (packetCount - 1) {
			le = lastPacketSize
		}

		data := make([]byte, 4+le)
		binary.BigEndian.PutUint16(data, 16)
		binary.BigEndian.PutUint16(data[2:], uint16(le))
		copy(data[4:], au[:le])
		au = au[le:]

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

func (e *Encoder) lenAggregated(ats []*AUAndTimestamp, additionalEl *AUAndTimestamp) int {
	ret := 2 // AU-headers-length

	for _, at := range ats {
		ret += 2          // AU-header
		ret += len(at.AU) // AU
	}

	if additionalEl != nil {
		ret += 2                    // AU-header
		ret += len(additionalEl.AU) // AU
	}

	return ret
}

func (e *Encoder) writeAggregated(ats []*AUAndTimestamp) ([][]byte, error) {
	payload := make([]byte, e.lenAggregated(ats, nil))

	// AU-headers-length
	binary.BigEndian.PutUint16(payload, uint16(len(ats)*16))
	pos := 2

	// AU-headers
	for _, at := range ats {
		binary.BigEndian.PutUint16(payload[pos:], uint16(len(at.AU))<<3)
		pos += 2
	}

	// AUs
	for _, at := range ats {
		auLen := copy(payload[pos:], at.AU)
		pos += auLen
	}

	rpkt := rtp.Packet{
		Header: rtp.Header{
			Version:        rtpVersion,
			PayloadType:    e.payloadType,
			SequenceNumber: e.sequenceNumber,
			Timestamp:      e.encodeTimestamp(ats[0].Timestamp),
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
