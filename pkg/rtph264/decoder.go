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

// ErrNonStartingPacketAndNoPrevious is returned when we decoded a non-starting
// packet of a fragmented NALU and we didn't received anything before.
// It's normal to receive this when we are decoding a stream that has been already
// running for some time.
var ErrNonStartingPacketAndNoPrevious = errors.New("decoded a non-starting fragmented packet without any previous starting packet")

// PacketConnReader creates a io.Reader around a net.PacketConn.
type PacketConnReader struct {
	net.PacketConn
}

// Read implements io.Reader.
func (r PacketConnReader) Read(p []byte) (int, error) {
	n, _, err := r.PacketConn.ReadFrom(p)
	return n, err
}

// Decoder is a RTP/H264 decoder.
type Decoder struct {
	initialTs    uint32
	initialTsSet bool

	// for Decode()
	startingPacketReceived bool
	isDecodingFragmented   bool
	fragmentedBuf          []byte
}

// NewDecoder allocates a Decoder.
func NewDecoder() *Decoder {
	return &Decoder{}
}

func (d *Decoder) decodeTimestamp(ts uint32) time.Duration {
	return (time.Duration(ts) - time.Duration(d.initialTs)) * time.Second / rtpClockRate
}

// Decode decodes NALUs from a RTP/H264 packet.
// It returns the decoded NALUs and their PTS.
func (d *Decoder) Decode(byts []byte) ([][]byte, time.Duration, error) {
	pkt := rtp.Packet{}
	err := pkt.Unmarshal(byts)
	if err != nil {
		d.isDecodingFragmented = false
		return nil, 0, err
	}

	return d.DecodeRTP(&pkt)
}

// DecodeRTP decodes NALUs from a rtp.Packet.
func (d *Decoder) DecodeRTP(pkt *rtp.Packet) ([][]byte, time.Duration, error) {
	if !d.isDecodingFragmented {
		if !d.initialTsSet {
			d.initialTsSet = true
			d.initialTs = pkt.Timestamp
		}

		if len(pkt.Payload) < 1 {
			return nil, 0, fmt.Errorf("payload is too short")
		}

		typ := naluType(pkt.Payload[0] & 0x1F)

		switch typ {
		case naluTypeSTAPA:
			var nalus [][]byte
			pkt.Payload = pkt.Payload[1:]

			for len(pkt.Payload) > 0 {
				if len(pkt.Payload) < 2 {
					return nil, 0, fmt.Errorf("invalid STAP-A packet (invalid size)")
				}

				size := binary.BigEndian.Uint16(pkt.Payload)
				pkt.Payload = pkt.Payload[2:]

				// avoid final padding
				if size == 0 {
					break
				}

				if int(size) > len(pkt.Payload) {
					return nil, 0, fmt.Errorf("invalid STAP-A packet (invalid size)")
				}

				nalus = append(nalus, pkt.Payload[:size])
				pkt.Payload = pkt.Payload[size:]
			}

			if len(nalus) == 0 {
				return nil, 0, fmt.Errorf("STAP-A packet doesn't contain any NALU")
			}

			d.startingPacketReceived = true
			return nalus, d.decodeTimestamp(pkt.Timestamp), nil

		case naluTypeFUA: // first packet of a fragmented NALU
			if len(pkt.Payload) < 2 {
				return nil, 0, fmt.Errorf("invalid FU-A packet (invalid size)")
			}

			start := pkt.Payload[1] >> 7
			if start != 1 {
				if !d.startingPacketReceived {
					return nil, 0, ErrNonStartingPacketAndNoPrevious
				}
				return nil, 0, fmt.Errorf("invalid FU-A packet (non-starting)")
			}

			nri := (pkt.Payload[0] >> 5) & 0x03
			typ := pkt.Payload[1] & 0x1F
			d.fragmentedBuf = append([]byte{(nri << 5) | typ}, pkt.Payload[2:]...)

			d.isDecodingFragmented = true
			d.startingPacketReceived = true
			return nil, 0, ErrMorePacketsNeeded

		case naluTypeSTAPB, naluTypeMTAP16,
			naluTypeMTAP24, naluTypeFUB:
			return nil, 0, fmt.Errorf("packet type not supported (%v)", typ)
		}

		d.startingPacketReceived = true
		return [][]byte{pkt.Payload}, d.decodeTimestamp(pkt.Timestamp), nil
	}

	// we are decoding a fragmented NALU

	if len(pkt.Payload) < 2 {
		d.isDecodingFragmented = false
		return nil, 0, fmt.Errorf("invalid FU-A packet (invalid size)")
	}

	typ := naluType(pkt.Payload[0] & 0x1F)
	if typ != naluTypeFUA {
		d.isDecodingFragmented = false
		return nil, 0, fmt.Errorf("expected FU-A packet, got another type")
	}

	start := pkt.Payload[1] >> 7
	end := (pkt.Payload[1] >> 6) & 0x01

	if start == 1 {
		d.isDecodingFragmented = false
		return nil, 0, fmt.Errorf("invalid FU-A packet (decoded two starting packets in a row)")
	}

	d.fragmentedBuf = append(d.fragmentedBuf, pkt.Payload[2:]...)

	if end != 1 {
		return nil, 0, ErrMorePacketsNeeded
	}

	d.isDecodingFragmented = false
	d.startingPacketReceived = true
	return [][]byte{d.fragmentedBuf}, d.decodeTimestamp(pkt.Timestamp), nil
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
			switch naluType(nalu[0] & 0x1F) {
			case naluTypeSPS:
				sps = append([]byte(nil), nalu...)
				if sps != nil && pps != nil {
					return sps, pps, nil
				}

			case naluTypePPS:
				pps = append([]byte(nil), nalu...)
				if sps != nil && pps != nil {
					return sps, pps, nil
				}
			}
		}
	}
}
