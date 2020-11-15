package rtph264

import (
	"math/rand"
	"time"

	"github.com/pion/rtp"
)

const (
	rtpVersion        = 0x02
	rtpPayloadMaxSize = 1460 // 1500 - ip header - udp header - rtp header
)

// Encoder is a RTP/H264 encoder.
type Encoder struct {
	payloadType    uint8
	sequenceNumber uint16
	ssrc           uint32
	initialTs      uint32
	started        time.Duration
}

// NewEncoder allocates an Encoder.
func NewEncoder(relativeType uint8) (*Encoder, error) {
	return &Encoder{
		payloadType:    96 + relativeType,
		sequenceNumber: uint16(rand.Uint32()),
		ssrc:           rand.Uint32(),
		initialTs:      rand.Uint32(),
	}, nil
}

// Write encodes NALUs into RTP/H264 packets.
func (e *Encoder) Write(nalus [][]byte, timestamp time.Duration) ([][]byte, error) {
	if e.started == 0 {
		e.started = timestamp
	}

	// rtp/h264 uses a 90khz clock
	rtpTs := e.initialTs + uint32((timestamp-e.started).Seconds()*90000)

	var frames [][]byte

	for i, nalu := range nalus {
		naluFrames, err := e.writeNalu(nalu, rtpTs, (i == len(nalus)-1))
		if err != nil {
			return nil, err
		}
		frames = append(frames, naluFrames...)
	}

	return frames, nil
}

func (e *Encoder) writeNalu(nalu []byte, rtpTs uint32, isFinal bool) ([][]byte, error) {
	// if the NALU fits into a single RTP packet, use a single NALU payload
	if len(nalu) < rtpPayloadMaxSize {
		return e.writeSingle(nalu, rtpTs, isFinal)
	}

	// otherwise, split the NALU into multiple fragmentation payloads
	return e.writeFragmented(nalu, rtpTs, isFinal)
}

func (e *Encoder) writeSingle(nalu []byte, rtpTs uint32, isFinal bool) ([][]byte, error) {
	rpkt := rtp.Packet{
		Header: rtp.Header{
			Version:        rtpVersion,
			PayloadType:    e.payloadType,
			SequenceNumber: e.sequenceNumber,
			Timestamp:      rtpTs,
			SSRC:           e.ssrc,
		},
		Payload: nalu,
	}
	e.sequenceNumber++

	if isFinal {
		rpkt.Header.Marker = true
	}

	frame, err := rpkt.Marshal()
	if err != nil {
		return nil, err
	}

	return [][]byte{frame}, nil
}

func (e *Encoder) writeFragmented(nalu []byte, rtpTs uint32, isFinal bool) ([][]byte, error) {
	// use only FU-A, not FU-B, since we always use non-interleaved mode
	// (packetization-mode=1)
	frameCount := (len(nalu) - 1) / (rtpPayloadMaxSize - 2)
	lastFrameSize := (len(nalu) - 1) % (rtpPayloadMaxSize - 2)
	if lastFrameSize > 0 {
		frameCount++
	}
	frames := make([][]byte, frameCount)

	nri := (nalu[0] >> 5) & 0x03
	typ := nalu[0] & 0x1F
	nalu = nalu[1:] // remove header

	for i := 0; i < frameCount; i++ {
		indicator := (nri << 5) | uint8(NALUTypeFuA)

		start := uint8(0)
		if i == 0 {
			start = 1
		}
		end := uint8(0)
		le := rtpPayloadMaxSize - 2
		if i == (len(frames) - 1) {
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
				Timestamp:      rtpTs,
				SSRC:           e.ssrc,
			},
			Payload: data,
		}
		e.sequenceNumber++

		if isFinal && i == (len(frames)-1) {
			rpkt.Header.Marker = true
		}

		frame, err := rpkt.Marshal()
		if err != nil {
			return nil, err
		}

		frames[i] = frame
	}

	return frames, nil
}
