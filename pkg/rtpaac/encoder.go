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
// It returns the encoded packets.
func (e *Encoder) Encode(aus [][]byte, firstPTS time.Duration) ([][]byte, error) {
	var rets [][]byte
	var batch [][]byte

	pts := firstPTS

	// split AUs into batches
	for _, au := range aus {
		if e.lenAggregated(batch, au) <= rtpPayloadMaxSize {
			// add to existing batch
			batch = append(batch, au)
		} else {
			// write last batch
			if batch != nil {
				pkts, err := e.writeBatch(batch, pts)
				if err != nil {
					return nil, err
				}
				rets = append(rets, pkts...)
				pts += time.Duration(len(batch)) * 1000 * time.Second / time.Duration(e.clockRate)
			}

			// initialize new batch
			batch = [][]byte{au}
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

func (e *Encoder) writeBatch(aus [][]byte, firstPTS time.Duration) ([][]byte, error) {
	if len(aus) == 1 {
		// the AU fits into a single RTP packet
		if len(aus[0]) < rtpPayloadMaxSize {
			return e.writeAggregated(aus, firstPTS)
		}

		// split the AU into multiple fragmentation packet
		return e.writeFragmented(aus[0], firstPTS)
	}

	return e.writeAggregated(aus, firstPTS)
}

func (e *Encoder) writeFragmented(au []byte, pts time.Duration) ([][]byte, error) {
	packetCount := len(au) / (rtpPayloadMaxSize - 4)
	lastPacketSize := len(au) % (rtpPayloadMaxSize - 4)
	if lastPacketSize > 0 {
		packetCount++
	}

	ret := make([][]byte, packetCount)
	encPTS := e.encodeTimestamp(pts)

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
				Timestamp:      encPTS,
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

func (e *Encoder) lenAggregated(aus [][]byte, addAU []byte) int {
	ret := 2 // AU-headers-length

	for _, au := range aus {
		ret += 2       // AU-header
		ret += len(au) // AU
	}

	if addAU != nil {
		ret += 2          // AU-header
		ret += len(addAU) // AU
	}

	return ret
}

func (e *Encoder) writeAggregated(aus [][]byte, firstPTS time.Duration) ([][]byte, error) {
	payload := make([]byte, e.lenAggregated(aus, nil))

	// AU-headers-length
	binary.BigEndian.PutUint16(payload, uint16(len(aus)*16))
	pos := 2

	// AU-headers
	for _, au := range aus {
		binary.BigEndian.PutUint16(payload[pos:], uint16(len(au))<<3)
		pos += 2
	}

	// AUs
	for _, au := range aus {
		auLen := copy(payload[pos:], au)
		pos += auLen
	}

	rpkt := rtp.Packet{
		Header: rtp.Header{
			Version:        rtpVersion,
			PayloadType:    e.payloadType,
			SequenceNumber: e.sequenceNumber,
			Timestamp:      e.encodeTimestamp(firstPTS),
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
