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
	initialTs    uint32
	initialTsSet bool

	// for Decode() and FU-A
	state         decoderState
	fragmentedBuf []byte

	// for Read()
	nalusQueue []*NALUAndTimestamp
}

// NewDecoder allocates a Decoder.
func NewDecoder() *Decoder {
	return &Decoder{}
}

func (d *Decoder) decodeTimestamp(ts uint32) time.Duration {
	return (time.Duration(ts) - time.Duration(d.initialTs)) * time.Second / rtpClockRate
}

// Decode decodes NALUs from RTP/H264 packets.
// It can return:
// * no NALUs and ErrMorePacketsNeeded
// * one NALU (in case of FU-A)
// * multiple NALUs (in case of STAP-A)
func (d *Decoder) Decode(byts []byte) ([]*NALUAndTimestamp, error) {
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
			return []*NALUAndTimestamp{{
				NALU:      pkt.Payload,
				Timestamp: d.decodeTimestamp(pkt.Timestamp),
			}}, nil

		case NALUTypeStapA:
			var ret []*NALUAndTimestamp
			pkt.Payload = pkt.Payload[1:]

			for len(pkt.Payload) > 0 {
				if len(pkt.Payload) < 2 {
					return nil, fmt.Errorf("Invalid STAP-A packet")
				}

				size := binary.BigEndian.Uint16(pkt.Payload)
				pkt.Payload = pkt.Payload[2:]

				// avoid final padding
				if size == 0 {
					break
				}

				if int(size) > len(pkt.Payload) {
					return nil, fmt.Errorf("Invalid STAP-A packet")
				}

				ret = append(ret, &NALUAndTimestamp{
					NALU:      pkt.Payload[:size],
					Timestamp: d.decodeTimestamp(pkt.Timestamp),
				})
				pkt.Payload = pkt.Payload[size:]
			}

			if len(ret) == 0 {
				return nil, fmt.Errorf("STAP-A packet doesn't contain any NALU")
			}

			return ret, nil

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

		case NALUTypeStapB, NALUTypeMtap16, NALUTypeMtap24, NALUTypeFuB:
			return nil, fmt.Errorf("NALU type not supported (%v)", typ)
		}

		return nil, fmt.Errorf("invalid NALU type (%v)", typ)

	default: // decoderStateReadingFragmented
		pkt := rtp.Packet{}
		err := pkt.Unmarshal(byts)
		if err != nil {
			d.state = decoderStateInitial
			return nil, err
		}

		if len(pkt.Payload) < 2 {
			d.state = decoderStateInitial
			return nil, fmt.Errorf("Invalid FU-A packet")
		}

		typ := NALUType(pkt.Payload[0] & 0x1F)
		if typ != NALUTypeFuA {
			d.state = decoderStateInitial
			return nil, fmt.Errorf("non-starting NALU is not FU-A")
		}
		end := (pkt.Payload[1] >> 6) & 0x01

		d.fragmentedBuf = append(d.fragmentedBuf, pkt.Payload[2:]...)

		if end != 1 {
			return nil, ErrMorePacketsNeeded
		}

		d.state = decoderStateInitial
		return []*NALUAndTimestamp{{
			NALU:      d.fragmentedBuf,
			Timestamp: d.decodeTimestamp(pkt.Timestamp),
		}}, nil
	}
}

// Read reads RTP/H264 packets from a reader until a NALU is decoded.
func (d *Decoder) Read(r io.Reader) (*NALUAndTimestamp, error) {
	if len(d.nalusQueue) > 0 {
		nalu := d.nalusQueue[0]
		d.nalusQueue = d.nalusQueue[1:]
		return nalu, nil
	}

	buf := make([]byte, 2048)
	for {
		n, err := r.Read(buf)
		if err != nil {
			return nil, err
		}

		nalus, err := d.Decode(buf[:n])
		if err != nil {
			if err == ErrMorePacketsNeeded {
				continue
			}
			return nil, err
		}

		nalu := nalus[0]
		d.nalusQueue = nalus[1:]

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
