package rtph264

import (
	"math/rand"
	"time"

	"github.com/pion/rtp"
)

const (
	rtpVersion        = 0x02
	rtpPayloadMaxSize = 1460 // 1500 (mtu) - 20 (ip header) - 8 (udp header) - 12 (rtp header)
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
func NewEncoder(payloadType uint8) (*Encoder, error) {
	return &Encoder{
		payloadType:    payloadType,
		sequenceNumber: uint16(rand.Uint32()),
		ssrc:           rand.Uint32(),
		initialTs:      rand.Uint32(),
	}, nil
}

// Write encodes NALUs into RTP/H264 packets.
func (e *Encoder) Write(ts time.Duration, nalus [][]byte) ([][]byte, error) {
	if e.started == 0 {
		e.started = ts
	}

	// rtp/h264 uses a 90khz clock
	rtpTime := e.initialTs + uint32((ts-e.started).Seconds()*90000)

	var frames [][]byte

	for i, nalu := range nalus {
		naluFrames, err := e.writeNALU(rtpTime, nalu, (i == len(nalus)-1))
		if err != nil {
			return nil, err
		}
		frames = append(frames, naluFrames...)
	}

	return frames, nil
}

func (e *Encoder) writeNALU(rtpTime uint32, nalu []byte, isFinal bool) ([][]byte, error) {
	// if the NALU fits into a single RTP packet, use a single NALU payload
	if len(nalu) < rtpPayloadMaxSize {
		return e.writeSingle(rtpTime, nalu, isFinal)
	}

	// otherwise, split the NALU into multiple fragmentation payloads
	return e.writeFragmented(rtpTime, nalu, isFinal)
}

func (e *Encoder) writeSingle(rtpTime uint32, nalu []byte, isFinal bool) ([][]byte, error) {
	rpkt := rtp.Packet{
		Header: rtp.Header{
			Version:        rtpVersion,
			PayloadType:    e.payloadType,
			SequenceNumber: e.sequenceNumber,
			Timestamp:      rtpTime,
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

func (e *Encoder) writeFragmented(rtpTime uint32, nalu []byte, isFinal bool) ([][]byte, error) {
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
				Timestamp:      rtpTime,
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
