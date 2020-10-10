// Package rtph264 contains a RTP/H264 decoder and encoder.
package rtph264

import (
	"fmt"
	"io"
	"net"

	"github.com/pion/rtp"
)

type packetConnReader struct {
	inner net.PacketConn
}

func (r packetConnReader) Read(p []byte) (int, error) {
	n, _, err := r.inner.ReadFrom(p)
	return n, err
}

// Decoder is a RTP/H264 decoder.
type Decoder struct {
	r   io.Reader
	buf []byte
}

// NewDecoder creates a decoder around a Reader.
func NewDecoder(r io.Reader) *Decoder {
	return &Decoder{
		r:   r,
		buf: make([]byte, 2048),
	}
}

// NewDecoderFromPacketConn creates a decoder around a net.PacketConn.
func NewDecoderFromPacketConn(pc net.PacketConn) *Decoder {
	return NewDecoder(packetConnReader{pc})
}

// Read decodes NALUs from RTP/H264 packets.
func (d *Decoder) Read() ([][]byte, error) {
	n, err := d.r.Read(d.buf)
	if err != nil {
		return nil, err
	}

	pkt := rtp.Packet{}
	err = pkt.Unmarshal(d.buf[:n])
	if err != nil {
		return nil, err
	}
	payload := pkt.Payload

	typ := naluType(payload[0] & 0x1F)

	if typ >= naluTypeFirstSingle && typ <= naluTypeLastSingle {
		return [][]byte{payload}, nil
	}

	switch typ {
	case naluTypeFuA:
		return d.readFragmented(payload)

	case naluTypeStapA, naluTypeStapB, naluTypeMtap16, naluTypeMtap24, naluTypeFuB:
		return nil, fmt.Errorf("NALU type not supported (%d)", typ)
	}

	return nil, fmt.Errorf("invalid NALU type (%d)", typ)
}

func (d *Decoder) readFragmented(payload []byte) ([][]byte, error) {
	// A NALU can have any size; we can't preallocate it
	var ret []byte

	// process first nalu
	nri := (payload[0] >> 5) & 0x03
	start := payload[1] >> 7
	if start != 1 {
		return nil, fmt.Errorf("first NALU does not contain the start bit")
	}
	typ := payload[1] & 0x1F
	ret = append([]byte{(nri << 5) | typ}, payload[2:]...)

	// process other nalus
	for {
		n, err := d.r.Read(d.buf)
		if err != nil {
			return nil, err
		}

		pkt := rtp.Packet{}
		err = pkt.Unmarshal(d.buf[:n])
		if err != nil {
			return nil, err
		}
		payload := pkt.Payload

		typ := naluType(payload[0] & 0x1F)
		if typ != naluTypeFuA {
			return nil, fmt.Errorf("non-starting NALU is not FU-A")
		}
		end := (payload[1] >> 6) & 0x01

		ret = append(ret, payload[2:]...)

		if end == 1 {
			break
		}
	}

	return [][]byte{ret}, nil
}

// ReadSPSPPS decodes NALUs until SPS and PPS are found.
func (d *Decoder) ReadSPSPPS() ([]byte, []byte, error) {
	var sps []byte
	var pps []byte

	for {
		nalus, err := d.Read()
		if err != nil {
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
