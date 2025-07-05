package mikey

import "fmt"

func boolToUint8(v bool) uint8 {
	if v {
		return 1
	}
	return 0
}

// DataType is a message data type.
type DataType uint8

// RFC3830, Table 6.1.a
const (
	DataTypeInitiatorPSK DataType = 0
)

// CSIDMapType is a CS ID map type.
type CSIDMapType uint8

// RFC3830, Table 6.1.d
const (
	CSIDMapTypeSRTPID CSIDMapType = 0
)

// SRTPIDEntry is an entry of a SRTP-ID map.
type SRTPIDEntry struct {
	PolicyNo uint8
	SSRC     uint32
	ROC      uint32
}

// Header is a MIKEY header.
type Header struct {
	Version     uint8
	DataType    DataType
	V           bool
	PRFFunc     uint8
	CSBID       uint32
	CSIDMapType CSIDMapType
	CSIDMapInfo []SRTPIDEntry
}

func (h *Header) unmarshal(buf []byte) (int, payloadType, error) {
	if len(buf) < 10 {
		return 0, 0, fmt.Errorf("header too short")
	}

	n := 0
	h.Version = buf[n]
	n++

	if h.Version != 1 {
		return 0, 0, fmt.Errorf("unsupported version: %v", h.Version)
	}

	h.DataType = DataType(buf[n])
	n++

	if h.DataType != DataTypeInitiatorPSK {
		return 0, 0, fmt.Errorf("unsupported data type: %v", h.DataType)
	}

	nextPayload := payloadType(buf[n])
	n++

	h.V = (buf[n] >> 7) != 0
	h.PRFFunc = buf[n] & 0b01111111
	n++

	if h.V {
		return 0, 0, fmt.Errorf("unsupported V: %v", h.V)
	}

	if h.PRFFunc != 0 {
		return 0, 0, fmt.Errorf("unsupported PRFFunc: %v", h.PRFFunc)
	}

	h.CSBID = uint32(buf[n])<<24 | uint32(buf[n+1])<<16 | uint32(buf[n+2])<<8 | uint32(buf[n+3])
	n += 4

	numCS := buf[n]
	n++

	h.CSIDMapType = CSIDMapType(buf[n])
	n++

	if h.CSIDMapType != CSIDMapTypeSRTPID {
		return 0, 0, fmt.Errorf("unsupported map type: %d", h.CSIDMapType)
	}

	if len(buf[n:]) < (int(numCS) * 9) {
		return 0, 0, fmt.Errorf("header too short")
	}

	h.CSIDMapInfo = make([]SRTPIDEntry, numCS)

	for i := range numCS {
		h.CSIDMapInfo[i].PolicyNo = buf[n]
		n++
		h.CSIDMapInfo[i].SSRC = uint32(buf[n])<<24 | uint32(buf[n+1])<<16 | uint32(buf[n+2])<<8 | uint32(buf[n+3])
		n += 4
		h.CSIDMapInfo[i].ROC = uint32(buf[n])<<24 | uint32(buf[n+1])<<16 | uint32(buf[n+2])<<8 | uint32(buf[n+3])
		n += 4
	}

	return n, nextPayload, nil
}

func (h *Header) marshalSize() int {
	return 10 + len(h.CSIDMapInfo)*9
}

func (h *Header) marshalTo(buf []byte, nextPayload payloadType) (int, error) {
	buf[0] = h.Version
	buf[1] = byte(h.DataType)
	buf[2] = byte(nextPayload)
	buf[3] = boolToUint8(h.V)<<7 | h.PRFFunc
	buf[4] = byte(h.CSBID >> 24)
	buf[5] = byte(h.CSBID >> 16)
	buf[6] = byte(h.CSBID >> 8)
	buf[7] = byte(h.CSBID)
	buf[8] = byte(len(h.CSIDMapInfo))
	buf[9] = byte(h.CSIDMapType)
	n := 10

	for _, mi := range h.CSIDMapInfo {
		buf[n] = mi.PolicyNo
		buf[n+1] = byte(mi.SSRC >> 24)
		buf[n+2] = byte(mi.SSRC >> 16)
		buf[n+3] = byte(mi.SSRC >> 8)
		buf[n+4] = byte(mi.SSRC)
		buf[n+5] = byte(mi.ROC >> 24)
		buf[n+6] = byte(mi.ROC >> 16)
		buf[n+7] = byte(mi.ROC >> 8)
		buf[n+8] = byte(mi.ROC)
		n += 9
	}

	return n, nil
}
