package headers

import (
	"fmt"
)

// RestartMarker is a Restart Marker header.
type RestartMarker struct {
	Interval uint16
	Count    uint16
}

// Unmarshal decodes the header.
func (h *RestartMarker) Unmarshal(byts []byte) (int, error) {
	if len(byts) < 4 {
		return 0, fmt.Errorf("buffer is too short")
	}

	h.Interval = uint16(byts[0])<<8 | uint16(byts[1])
	h.Count = uint16(byts[2])<<8 | uint16(byts[3])
	return 4, nil
}

// Marshal encodes the header.
func (h RestartMarker) Marshal(byts []byte) []byte {
	byts = append(byts, []byte{byte(h.Interval >> 8), byte(h.Interval)}...)
	byts = append(byts, []byte{byte(h.Count >> 8), byte(h.Count)}...)
	return byts
}
