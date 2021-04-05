package rtph264

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/pion/rtp"
)

// ErrMorePacketsNeeded is returned when more packets are needed.
var ErrMorePacketsNeeded = errors.New("need more packets")

// PacketConnReader creates a io.Reader around a net.PacketConn.
type PacketConnReader struct {
	net.PacketConn
}

// Read implements io.Reader.
func (r PacketConnReader) Read(p []byte) (int, error) {
	n, _, err := r.PacketConn.ReadFrom(p)
	return n, err
}

type decoderState int

const (
	decoderStateInitial decoderState = iota
	decoderStateReadingFragmented
)

// Decoder is a RTP/H264 decoder.
type Decoder struct {
	initialTs    uint32
	initialTsSet bool

	// for Decode()
	state         decoderState
	fragmentedBuf []byte
}

// NewDecoder allocates a Decoder.
func NewDecoder() *Decoder {
	return &Decoder{}
}

func (d *Decoder) decodeTimestamp(ts uint32) time.Duration {
	return (time.Duration(ts) - time.Duration(d.initialTs)) * time.Second / rtpClockRate
}

// Decode decodes NALUs from a RTP/H264 packet.
// It can return:
// * no NALUs and ErrMorePacketsNeeded
// * one NALU (in case of FU-A)
// * multiple NALUs (in case of STAP-A)
func (d *Decoder) Decode(byts []byte) ([][]byte, time.Duration, error) {
	switch d.state {
	case decoderStateInitial:
		pkt := rtp.Packet{}
		err := pkt.Unmarshal(byts)
		if err != nil {
			return nil, 0, err
		}

		if !d.initialTsSet {
			d.initialTsSet = true
			d.initialTs = pkt.Timestamp
		}

		if len(pkt.Payload) < 1 {
			return nil, 0, fmt.Errorf("payload is too short")
		}

		typ := NALUType(pkt.Payload[0] & 0x1F)

		switch typ {
		case NALUTypeSTAPA:
			var nalus [][]byte
			pkt.Payload = pkt.Payload[1:]

			for len(pkt.Payload) > 0 {
				if len(pkt.Payload) < 2 {
					return nil, 0, fmt.Errorf("Invalid STAP-A packet")
				}

				size := binary.BigEndian.Uint16(pkt.Payload)
				pkt.Payload = pkt.Payload[2:]

				// avoid final padding
				if size == 0 {
					break
				}

				if int(size) > len(pkt.Payload) {
					return nil, 0, fmt.Errorf("Invalid STAP-A packet")
				}

				nalus = append(nalus, pkt.Payload[:size])
				pkt.Payload = pkt.Payload[size:]
			}

			if len(nalus) == 0 {
				return nil, 0, fmt.Errorf("STAP-A packet doesn't contain any NALU")
			}

			return nalus, d.decodeTimestamp(pkt.Timestamp), nil

		case NALUTypeFUA: // first packet of a fragmented NALU
			if len(pkt.Payload) < 2 {
				return nil, 0, fmt.Errorf("Invalid FU-A packet")
			}

			start := pkt.Payload[1] >> 7
			if start != 1 {
				return nil, 0, fmt.Errorf("first NALU does not contain the start bit")
			}

			nri := (pkt.Payload[0] >> 5) & 0x03
			typ := pkt.Payload[1] & 0x1F
			d.fragmentedBuf = append([]byte{(nri << 5) | typ}, pkt.Payload[2:]...)

			d.state = decoderStateReadingFragmented
			return nil, 0, ErrMorePacketsNeeded

		case NALUTypeSTAPB, NALUTypeMTAP16,
			NALUTypeMTAP24, NALUTypeFUB:
			return nil, 0, fmt.Errorf("NALU type not supported (%v)", typ)
		}

		return [][]byte{pkt.Payload}, d.decodeTimestamp(pkt.Timestamp), nil

	default: // decoderStateReadingFragmented
		pkt := rtp.Packet{}
		err := pkt.Unmarshal(byts)
		if err != nil {
			d.state = decoderStateInitial
			return nil, 0, err
		}

		if len(pkt.Payload) < 2 {
			d.state = decoderStateInitial
			return nil, 0, fmt.Errorf("Invalid FU-A packet")
		}

		typ := NALUType(pkt.Payload[0] & 0x1F)
		if typ != NALUTypeFUA {
			d.state = decoderStateInitial
			return nil, 0, fmt.Errorf("non-starting NALU is not FU-A")
		}

		end := (pkt.Payload[1] >> 6) & 0x01

		d.fragmentedBuf = append(d.fragmentedBuf, pkt.Payload[2:]...)

		if end != 1 {
			return nil, 0, ErrMorePacketsNeeded
		}

		d.state = decoderStateInitial
		return [][]byte{d.fragmentedBuf}, d.decodeTimestamp(pkt.Timestamp), nil
	}
}

// ReadSPSPPS reads RTP/H264 packets from a reader until SPS and PPS are
// found, and returns them.
func (d *Decoder) ReadSPSPPS(r io.Reader) ([]byte, []byte, error) {
	var sps []byte
	var pps []byte

	buf := make([]byte, 2048)
	for {
		n, err := r.Read(buf)
		if err != nil {
			return nil, nil, err
		}

		nalus, _, err := d.Decode(buf[:n])
		if err != nil {
			if err == ErrMorePacketsNeeded {
				continue
			}
			return nil, nil, err
		}

		for _, nalu := range nalus {
			switch NALUType(nalu[0] & 0x1F) {
			case NALUTypeSPS:
				sps = append([]byte(nil), nalu...)
				if sps != nil && pps != nil {
					return sps, pps, nil
				}

			case NALUTypePPS:
				pps = append([]byte(nil), nalu...)
				if sps != nil && pps != nil {
					return sps, pps, nil
				}
			}
		}
	}
}
