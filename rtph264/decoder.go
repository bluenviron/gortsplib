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

	typ := NALUType(payload[0] & 0x1F)

	switch typ {
	case NALUTypeNonIDR, NALUTypeDataPartitionA, NALUTypeDataPartitionB,
		NALUTypeDataPartitionC, NALUTypeIDR, NALUTypeSei, NALUTypeSPS,
		NALUTypePPS, NALUTypeAccessUnitDelimiter, NALUTypeEndOfSequence,
		NALUTypeEndOfStream, NALUTypeFillerData, NALUTypeSPSExtension,
		NALUTypePrefix, NALUTypeSubsetSPS, NALUTypeReserved16, NALUTypeReserved17,
		NALUTypeReserved18, NALUTypeSliceLayerWithoutPartitioning,
		NALUTypeSliceExtension, NALUTypeSliceExtensionDepth, NALUTypeReserved22,
		NALUTypeReserved23:
		return [][]byte{payload}, nil

	case NALUTypeFuA:
		return d.readFragmented(payload)

	case NALUTypeStapA, NALUTypeStapB, NALUTypeMtap16, NALUTypeMtap24, NALUTypeFuB:
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

		typ := NALUType(payload[0] & 0x1F)
		if typ != NALUTypeFuA {
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
