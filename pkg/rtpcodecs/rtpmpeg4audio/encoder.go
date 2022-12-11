package rtpmpeg4audio

import (
	"crypto/rand"
	"time"

	"github.com/pion/rtp"

	"github.com/aler9/gortsplib/v2/pkg/bits"
	"github.com/aler9/gortsplib/v2/pkg/mpeg4audio"
)

const (
	rtpVersion = 2
)

func randUint32() uint32 {
	var b [4]byte
	rand.Read(b[:])
	return uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3])
}

// Encoder is a RTP/MPEG4-audio encoder.
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

	// sample rate of packets.
	SampleRate int

	// The number of bits on which the AU-size field is encoded in the AU-header.
	SizeLength int

	// The number of bits on which the AU-Index is encoded in the first AU-header.
	IndexLength int

	// The number of bits on which the AU-Index-delta field is encoded in any non-first AU-header.
	IndexDeltaLength int

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
	return *e.InitialTimestamp + uint32(ts.Seconds()*float64(e.SampleRate))
}

// Encode encodes AUs into RTP/MPEG4-audio packets.
func (e *Encoder) Encode(aus [][]byte, pts time.Duration) ([]*rtp.Packet, error) {
	var rets []*rtp.Packet
	var batch [][]byte

	// split AUs into batches
	for _, au := range aus {
		if e.lenAggregated(batch, au) <= e.PayloadMaxSize {
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
				pts += time.Duration(len(batch)) * mpeg4audio.SamplesPerAccessUnit * time.Second / time.Duration(e.SampleRate)
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

func (e *Encoder) writeBatch(aus [][]byte, pts time.Duration) ([]*rtp.Packet, error) {
	if len(aus) == 1 {
		// the AU fits into a single RTP packet
		if len(aus[0]) < e.PayloadMaxSize {
			return e.writeAggregated(aus, pts)
		}

		// split the AU into multiple fragmentation packet
		return e.writeFragmented(aus[0], pts)
	}

	return e.writeAggregated(aus, pts)
}

func (e *Encoder) writeFragmented(au []byte, pts time.Duration) ([]*rtp.Packet, error) {
	auHeadersLen := e.SizeLength + e.IndexLength
	auHeadersLenBytes := auHeadersLen / 8
	if (auHeadersLen % 8) != 0 {
		auHeadersLenBytes++
	}
	auMaxSize := e.PayloadMaxSize - 2 - auHeadersLenBytes
	packetCount := len(au) / auMaxSize
	lastPacketSize := len(au) % auMaxSize
	if lastPacketSize > 0 {
		packetCount++
	}

	ret := make([]*rtp.Packet, packetCount)
	encPTS := e.encodeTimestamp(pts)

	for i := range ret {
		var le int
		if i != (packetCount - 1) {
			le = auMaxSize
		} else {
			le = lastPacketSize
		}

		byts := make([]byte, 2+auHeadersLenBytes+le)

		// AU-headers-length
		byts[0] = byte(auHeadersLen >> 8)
		byts[1] = byte(auHeadersLen)

		// AU-headers
		pos := 0
		bits.WriteBits(byts[2:], &pos, uint64(le), e.SizeLength)
		bits.WriteBits(byts[2:], &pos, 0, e.IndexLength)

		// AU
		copy(byts[2+auHeadersLenBytes:], au[:le])
		au = au[le:]

		ret[i] = &rtp.Packet{
			Header: rtp.Header{
				Version:        rtpVersion,
				PayloadType:    e.PayloadType,
				SequenceNumber: e.sequenceNumber,
				Timestamp:      encPTS,
				SSRC:           *e.SSRC,
				Marker:         (i == (packetCount - 1)),
			},
			Payload: byts,
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

func (e *Encoder) writeAggregated(aus [][]byte, pts time.Duration) ([]*rtp.Packet, error) {
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
			Timestamp:      e.encodeTimestamp(pts),
			SSRC:           *e.SSRC,
			Marker:         true,
		},
		Payload: payload,
	}

	e.sequenceNumber++

	return []*rtp.Packet{pkt}, nil
}
