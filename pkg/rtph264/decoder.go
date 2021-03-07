package rtph264

import (
	"errors"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/pion/rtp"
)

// ErrMorePacketsNeeded is returned by Decoder.Read when more packets are needed.
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
	initialTs     uint32
	initialTsSet  bool
	state         decoderState
	fragmentedBuf []byte
}

// NewDecoder allocates a Decoder.
func NewDecoder() *Decoder {
	return &Decoder{}
}

// Decode decodes a NALU from RTP/H264 packets.
// Since a NALU can require multiple RTP/H264 packets, it returns
// one packet, or no packets with ErrMorePacketsNeeded.
func (d *Decoder) Decode(byts []byte) (*NALUAndTimestamp, error) {
	switch d.state {
	case decoderStateInitial:
		pkt := rtp.Packet{}
		err := pkt.Unmarshal(byts)
		if err != nil {
			return nil, err
		}

		if !d.initialTsSet {
			d.initialTsSet = true
			d.initialTs = pkt.Timestamp
		}

		typ := NALUType(pkt.Payload[0] & 0x1F)

		switch typ {
		case NALUTypeNonIDR, NALUTypeDataPartitionA, NALUTypeDataPartitionB,
			NALUTypeDataPartitionC, NALUTypeIDR, NALUTypeSei, NALUTypeSPS,
			NALUTypePPS, NALUTypeAccessUnitDelimiter, NALUTypeEndOfSequence,
			NALUTypeEndOfStream, NALUTypeFillerData, NALUTypeSPSExtension,
			NALUTypePrefix, NALUTypeSubsetSPS, NALUTypeReserved16, NALUTypeReserved17,
			NALUTypeReserved18, NALUTypeSliceLayerWithoutPartitioning,
			NALUTypeSliceExtension, NALUTypeSliceExtensionDepth, NALUTypeReserved22,
			NALUTypeReserved23:
			return &NALUAndTimestamp{
				NALU:      pkt.Payload,
				Timestamp: time.Duration(pkt.Timestamp-d.initialTs) * time.Second / rtpClockRate,
			}, nil

		case NALUTypeFuA: // first packet of a fragmented NALU
			nri := (pkt.Payload[0] >> 5) & 0x03
			start := pkt.Payload[1] >> 7
			if start != 1 {
				return nil, fmt.Errorf("first NALU does not contain the start bit")
			}
			typ := pkt.Payload[1] & 0x1F
			d.fragmentedBuf = append([]byte{(nri << 5) | typ}, pkt.Payload[2:]...)

			d.state = decoderStateReadingFragmented
			return nil, ErrMorePacketsNeeded

		case NALUTypeStapA, NALUTypeStapB, NALUTypeMtap16, NALUTypeMtap24, NALUTypeFuB:
			return nil, fmt.Errorf("NALU type not supported (%d)", typ)
		}

		return nil, fmt.Errorf("invalid NALU type (%d)", typ)

	default: // decoderStateReadingFragmented
		pkt := rtp.Packet{}
		err := pkt.Unmarshal(byts)
		if err != nil {
			return nil, err
		}

		typ := NALUType(pkt.Payload[0] & 0x1F)
		if typ != NALUTypeFuA {
			return nil, fmt.Errorf("non-starting NALU is not FU-A")
		}
		end := (pkt.Payload[1] >> 6) & 0x01

		d.fragmentedBuf = append(d.fragmentedBuf, pkt.Payload[2:]...)

		if end != 1 {
			return nil, ErrMorePacketsNeeded
		}

		d.state = decoderStateInitial
		return &NALUAndTimestamp{
			NALU:      d.fragmentedBuf,
			Timestamp: time.Duration(pkt.Timestamp-d.initialTs) * time.Second / rtpClockRate,
		}, nil
	}
}

// Read reads RTP/H264 packets from a reader until a NALU is decoded.
func (d *Decoder) Read(r io.Reader) (*NALUAndTimestamp, error) {
	buf := make([]byte, 2048)
	for {
		n, err := r.Read(buf)
		if err != nil {
			return nil, err
		}

		nalu, err := d.Decode(buf[:n])
		if err != nil {
			if err == ErrMorePacketsNeeded {
				continue
			}
			return nil, err
		}
		return nalu, nil
	}
}

// ReadSPSPPS reads RTP/H264 packets from a reader until SPS and PPS are
// found, and returns them.
func (d *Decoder) ReadSPSPPS(r io.Reader) ([]byte, []byte, error) {
	var sps []byte
	var pps []byte

	for {
		nt, err := d.Read(r)
		if err != nil {
			return nil, nil, err
		}

		switch NALUType(nt.NALU[0] & 0x1F) {
		case NALUTypeSPS:
			sps = append([]byte(nil), nt.NALU...)
			if sps != nil && pps != nil {
				return sps, pps, nil
			}

		case NALUTypePPS:
			pps = append([]byte(nil), nt.NALU...)
			if sps != nil && pps != nil {
				return sps, pps, nil
			}
		}
	}
}
