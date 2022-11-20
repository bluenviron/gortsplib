package gortsplib

import (
	"github.com/pion/rtp"

	"github.com/aler9/gortsplib/pkg/h264"
)

// find IDR NALUs without decoding RTP
func rtpH264ContainsIDRorSPSorPPS(pkt *rtp.Packet) bool {
	if len(pkt.Payload) == 0 {
		return false
	}

	typ := h264.NALUType(pkt.Payload[0] & 0x1F)

	switch typ {
	case h264.NALUTypeIDR, h264.NALUTypeSPS, h264.NALUTypePPS:
		return true

	case 24: // STAP-A
		payload := pkt.Payload[1:]

		for len(payload) > 0 {
			if len(payload) < 2 {
				return false
			}

			size := uint16(payload[0])<<8 | uint16(payload[1])
			payload = payload[2:]

			if size == 0 || int(size) > len(payload) {
				return false
			}

			nalu := payload[:size]
			payload = payload[size:]

			typ = h264.NALUType(nalu[0] & 0x1F)
			switch typ {
			case h264.NALUTypeIDR, h264.NALUTypeSPS, h264.NALUTypePPS:
				return true
			}
		}

		return false

	case 28: // FU-A
		if len(pkt.Payload) < 2 {
			return false
		}

		start := pkt.Payload[1] >> 7
		if start != 1 {
			return false
		}

		typ := h264.NALUType(pkt.Payload[1] & 0x1F)
		switch typ {
		case h264.NALUTypeIDR, h264.NALUTypeSPS, h264.NALUTypePPS:
			return true
		}
		return false

	default:
		return false
	}
}

func ptsEqualsDTS(track Track, pkt *rtp.Packet) bool {
	if _, ok := track.(*TrackH264); ok {
		return rtpH264ContainsIDRorSPSorPPS(pkt)
	}

	return true
}
