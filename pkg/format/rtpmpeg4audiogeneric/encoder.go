package rtpmpeg4audiogeneric

import (
	"crypto/rand"

	"github.com/pion/rtp"

	"github.com/bluenviron/mediacommon/pkg/bits"
	"github.com/bluenviron/mediacommon/pkg/codecs/mpeg4audio"
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

func packetCount(avail, le int) int {
	n := le / avail
	if (le % avail) != 0 {
		n++
	}
	return n
}

// Encoder is a RTP/MPEG4-audio encoder.
// Specification: https://datatracker.ietf.org/doc/html/rfc3640
type Encoder struct {
	// payload type of packets.
	PayloadType uint8

	// The number of bits in which the AU-size field is encoded in the AU-header.
	SizeLength int

	// The number of bits in which the AU-Index is encoded in the first AU-header.
	IndexLength int

	// The number of bits in which the AU-Index-delta field is encoded in any non-first AU-header.
	IndexDeltaLength int

	// SSRC of packets (optional).
	// It defaults to a random value.
	SSRC *uint32

	// initial sequence number of packets (optional).
	// It defaults to a random value.
	InitialSequenceNumber *uint16

	// maximum size of packet payloads (optional).
	// It defaults to 1460.
	PayloadMaxSize int

	sequenceNumber uint16
}

// Init initializes the encoder.
func (e *Encoder) Init() error {
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

// Encode encodes AUs into RTP packets.
func (e *Encoder) Encode(aus [][]byte) ([]*rtp.Packet, error) {
	var rets []*rtp.Packet
	var batch [][]byte
	timestamp := uint32(0)

	// split AUs into batches
	for _, au := range aus {
		if e.lenAggregated(batch, au) <= e.PayloadMaxSize {
			// add to existing batch
			batch = append(batch, au)
		} else {
			// write current batch
			if batch != nil {
				pkts, err := e.writeBatch(batch, timestamp)
				if err != nil {
					return nil, err
				}
				rets = append(rets, pkts...)
				timestamp += uint32(len(batch)) * mpeg4audio.SamplesPerAccessUnit
			}

			// initialize new batch
			batch = [][]byte{au}
		}
	}

	// write last batch
	pkts, err := e.writeBatch(batch, timestamp)
	if err != nil {
		return nil, err
	}
	rets = append(rets, pkts...)

	return rets, nil
}

func (e *Encoder) writeBatch(aus [][]byte, timestamp uint32) ([]*rtp.Packet, error) {
	if len(aus) != 1 || e.lenAggregated(aus, nil) < e.PayloadMaxSize {
		return e.writeAggregated(aus, timestamp)
	}

	return e.writeFragmented(aus[0], timestamp)
}

func (e *Encoder) writeFragmented(au []byte, timestamp uint32) ([]*rtp.Packet, error) {
	auHeadersLen := e.SizeLength + e.IndexLength
	auHeadersLenBytes := auHeadersLen / 8
	if (auHeadersLen % 8) != 0 {
		auHeadersLenBytes++
	}

	avail := e.PayloadMaxSize - 2 - auHeadersLenBytes
	le := len(au)
	packetCount := packetCount(avail, le)

	ret := make([]*rtp.Packet, packetCount)
	le = avail

	for i := range ret {
		if i == (packetCount - 1) {
			le = len(au)
		}

		payload := make([]byte, 2+auHeadersLenBytes+le)

		// AU-headers-length
		payload[0] = byte(auHeadersLen >> 8)
		payload[1] = byte(auHeadersLen)

		// AU-headers
		pos := 0
		bits.WriteBits(payload[2:], &pos, uint64(le), e.SizeLength)
		bits.WriteBits(payload[2:], &pos, 0, e.IndexLength)

		// AU
		copy(payload[2+auHeadersLenBytes:], au)
		au = au[le:]

		ret[i] = &rtp.Packet{
			Header: rtp.Header{
				Version:        rtpVersion,
				PayloadType:    e.PayloadType,
				SequenceNumber: e.sequenceNumber,
				Timestamp:      timestamp,
				SSRC:           *e.SSRC,
				Marker:         (i == packetCount-1),
			},
			Payload: payload,
		}

		e.sequenceNumber++
	}

	return ret, nil
}

func (e *Encoder) lenAggregated(aus [][]byte, addAU []byte) int {
	ret := 2 // AU-headers-length

	// AU-headers
	auHeadersLen := 0
	i := 0
	for range aus {
		if i == 0 {
			auHeadersLen += e.SizeLength + e.IndexLength
		} else {
			auHeadersLen += e.SizeLength + e.IndexDeltaLength
		}
		i++
	}
	if addAU != nil {
		if i == 0 {
			auHeadersLen += e.SizeLength + e.IndexLength
		} else {
			auHeadersLen += e.SizeLength + e.IndexDeltaLength
		}
	}
	ret += auHeadersLen / 8
	if (auHeadersLen % 8) != 0 {
		ret++
	}

	// AU
	for _, au := range aus {
		ret += len(au)
	}
	ret += len(addAU)

	return ret
}

func (e *Encoder) writeAggregated(aus [][]byte, timestamp uint32) ([]*rtp.Packet, error) {
	payload := make([]byte, e.lenAggregated(aus, nil))

	// AU-headers
	written := 0
	pos := 0
	for i, au := range aus {
		bits.WriteBits(payload[2:], &pos, uint64(len(au)), e.SizeLength)
		written += e.SizeLength
		if i == 0 {
			bits.WriteBits(payload[2:], &pos, 0, e.IndexLength)
			written += e.IndexLength
		} else {
			bits.WriteBits(payload[2:], &pos, 0, e.IndexDeltaLength)
			written += e.IndexDeltaLength
		}
	}
	pos = 2 + (written / 8)
	if (written % 8) != 0 {
		pos++
	}

	// AU-headers-length
	payload[0] = byte(written >> 8)
	payload[1] = byte(written)

	// AUs
	for _, au := range aus {
		auLen := copy(payload[pos:], au)
		pos += auLen
	}

	pkt := &rtp.Packet{
		Header: rtp.Header{
			Version:        rtpVersion,
			PayloadType:    e.PayloadType,
			SequenceNumber: e.sequenceNumber,
			Timestamp:      timestamp,
			SSRC:           *e.SSRC,
			Marker:         true,
		},
		Payload: payload,
	}

	e.sequenceNumber++

	return []*rtp.Packet{pkt}, nil
}
