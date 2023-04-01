package headers

import (
	"fmt"
)

// JPEG is a JPEG header.
type JPEG struct {
	TypeSpecific   uint8
	FragmentOffset uint32
	Type           uint8
	Quantization   uint8
	Width          int
	Height         int
}

// Unmarshal decodes the header.
func (h *JPEG) Unmarshal(byts []byte) (int, error) {
	if len(byts) < 8 {
		return 0, fmt.Errorf("buffer is too short")
	}

	h.TypeSpecific = byts[0]
	h.FragmentOffset = uint32(byts[1])<<16 | uint32(byts[2])<<8 | uint32(byts[3])

	h.Type = byts[4]
	if h.Type != 1 {
		return 0, fmt.Errorf("Type %d is not supported", h.Type)
	}

	h.Quantization = byts[5]
	if h.Quantization != 255 {
		return 0, fmt.Errorf("Q %d is not supported", h.Quantization)
	}

	h.Width = int(byts[6]) * 8
	h.Height = int(byts[7]) * 8

	return 8, nil
}

// Marshal encodes the header.
func (h JPEG) Marshal(byts []byte) []byte {
	byts = append(byts, h.TypeSpecific)
	byts = append(byts, []byte{byte(h.FragmentOffset >> 16), byte(h.FragmentOffset >> 8), byte(h.FragmentOffset)}...)
	byts = append(byts, h.Type)
	byts = append(byts, h.Quantization)
	byts = append(byts, byte(h.Width/8))
	byts = append(byts, byte(h.Height/8))
	return byts
}
