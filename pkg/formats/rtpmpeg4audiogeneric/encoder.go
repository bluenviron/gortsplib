package rtpmpeg4audiogeneric

import (
	"crypto/rand"
	"time"

	"github.com/pion/rtp"

	"github.com/bluenviron/gortsplib/v3/pkg/rtptime"
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

// Encoder is a RTP/MPEG4-audio encoder.
// Specification: https://datatracker.ietf.org/doc/html/rfc3640
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

	// The number of bits in which the AU-size field is encoded in the AU-header.
	SizeLength int

	// The number of bits in which the AU-Index is encoded in the first AU-header.
	IndexLength int

	// The number of bits in which the AU-Index-delta field is encoded in any non-first AU-header.
	IndexDeltaLength int

	sequenceNumber uint16
	timeEncoder    *rtptime.Encoder
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
	e.timeEncoder = rtptime.NewEncoder(e.SampleRate, *e.InitialTimestamp)
	return nil
}

// Encode encodes AUs into RTP packets.
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
	if len(aus) != 1 || e.lenAggregated(aus, nil) < e.PayloadMaxSize {
		return e.writeAggregated(aus, pts)
	}

	return e.writeFragmented(aus[0], pts)
}

func (e *Encoder) writeFragmented(au []byte, pts time.Duration) ([]*rtp.Packet, error) {
	auHeadersLen := e.SizeLength + e.IndexLength
	auHeadersLenBytes := auHeadersLen / 8
	if (auHeadersLen % 8) != 0 {
		auHeadersLenBytes++
	}

	avail := e.PayloadMaxSize - 2 - auHeadersLenBytes
	le := len(au)
	packetCount := le / avail
	lastPacketSize := le % avail
	if lastPacketSize > 0 {
		packetCount++
	}

	ret := make([]*rtp.Packet, packetCount)
	encPTS := e.timeEncoder.Encode(pts)

	for i := range ret {
		var le int
		if i != (packetCount - 1) {
			le = avail
		} else {
			le = lastPacketSize
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
		copy(payload[2+auHeadersLenBytes:], au[:le])
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
			Timestamp:      e.timeEncoder.Encode(pts),
			SSRC:           *e.SSRC,
			Marker:         true,
		},
		Payload: payload,
	}

	e.sequenceNumber++

	return []*rtp.Packet{pkt}, nil
}
