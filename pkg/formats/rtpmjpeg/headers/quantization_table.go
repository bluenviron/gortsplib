package headers

import (
	"fmt"
)

// QuantizationTable is a Quantization Table header.
type QuantizationTable struct {
	MBZ       uint8
	Precision uint8
	Tables    []byte
}

// Unmarshal decodes the header.
func (h *QuantizationTable) Unmarshal(byts []byte) (int, error) {
	if len(byts) < 4 {
		return 0, fmt.Errorf("buffer is too short")
	}

	h.MBZ = byts[0]
	h.Precision = byts[1]
	if h.Precision != 0 {
		return 0, fmt.Errorf("Precision %d is not supported", h.Precision)
	}

	length := int(byts[2])<<8 | int(byts[3])
	switch length {
	case 64, 128:
	default:
		return 0, fmt.Errorf("Quantization table length %d is not supported", length)
	}

	if (len(byts) - 4) < length {
		return 0, fmt.Errorf("buffer is too short")
	}

	h.Tables = byts[4 : 4+length]
	return 4 + length, nil
}

// Marshal encodes the header.
func (h QuantizationTable) Marshal(byts []byte) []byte {
	byts = append(byts, h.MBZ)
	byts = append(byts, h.Precision)
	l := len(h.Tables)
	byts = append(byts, []byte{byte(l >> 8), byte(l)}...)
	byts = append(byts, h.Tables...)
	return byts
}
