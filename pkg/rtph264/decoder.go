package rtph264

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/pion/rtp"

	"github.com/aler9/gortsplib/pkg/codech264"
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

		typ := codech264.NALUType(pkt.Payload[0] & 0x1F)

		switch typ {
		case codech264.NALUTypeNonIDR, codech264.NALUTypeDataPartitionA, codech264.NALUTypeDataPartitionB,
			codech264.NALUTypeDataPartitionC, codech264.NALUTypeIDR, codech264.NALUTypeSei, codech264.NALUTypeSPS,
			codech264.NALUTypePPS, codech264.NALUTypeAccessUnitDelimiter, codech264.NALUTypeEndOfSequence,
			codech264.NALUTypeEndOfStream, codech264.NALUTypeFillerData, codech264.NALUTypeSPSExtension,
			codech264.NALUTypePrefix, codech264.NALUTypeSubsetSPS, codech264.NALUTypeReserved16, codech264.NALUTypeReserved17,
			codech264.NALUTypeReserved18, codech264.NALUTypeSliceLayerWithoutPartitioning,
			codech264.NALUTypeSliceExtension, codech264.NALUTypeSliceExtensionDepth, codech264.NALUTypeReserved22,
			codech264.NALUTypeReserved23:
			return []*NALUAndTimestamp{{
				NALU:      pkt.Payload,
				Timestamp: d.decodeTimestamp(pkt.Timestamp),
			}}, nil

		case codech264.NALUTypeStapA:
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

		case codech264.NALUTypeFuA: // first packet of a fragmented NALU
			start := pkt.Payload[1] >> 7
			if start != 1 {
				return nil, fmt.Errorf("first NALU does not contain the start bit")
			}

			nri := (pkt.Payload[0] >> 5) & 0x03
			typ := pkt.Payload[1] & 0x1F
			d.fragmentedBuf = append([]byte{(nri << 5) | typ}, pkt.Payload[2:]...)

			d.state = decoderStateReadingFragmented
			return nil, ErrMorePacketsNeeded

		case codech264.NALUTypeStapB, codech264.NALUTypeMtap16, codech264.NALUTypeMtap24, codech264.NALUTypeFuB:
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

		typ := codech264.NALUType(pkt.Payload[0] & 0x1F)
		if typ != codech264.NALUTypeFuA {
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

		switch codech264.NALUType(nt.NALU[0] & 0x1F) {
		case codech264.NALUTypeSPS:
			sps = append([]byte(nil), nt.NALU...)
			if sps != nil && pps != nil {
				return sps, pps, nil
			}

		case codech264.NALUTypePPS:
			pps = append([]byte(nil), nt.NALU...)
			if sps != nil && pps != nil {
				return sps, pps, nil
			}
		}
	}
}
