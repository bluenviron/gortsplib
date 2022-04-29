package h264

import (
	"encoding/binary"
	"fmt"
)

// AVCCDecode decodes NALUs from the AVCC stream format.
func AVCCDecode(byts []byte) ([][]byte, error) {
	var ret [][]byte

	for len(byts) > 0 {
		if len(byts) < 4 {
			return nil, fmt.Errorf("invalid length")
		}

		le := binary.BigEndian.Uint32(byts)
		byts = byts[4:]

		if len(byts) < int(le) {
			return nil, fmt.Errorf("invalid length")
		}

		ret = append(ret, byts[:le])
		byts = byts[le:]
	}

	if len(ret) == 0 {
		return nil, fmt.Errorf("no NALUs decoded")
	}

	return ret, nil
}

func avccEncodeSize(nalus [][]byte) int {
	n := 0
	for _, nalu := range nalus {
		n += 4 + len(nalu)
	}
	return n
}

// AVCCEncode encodes NALUs into the AVCC stream format.
func AVCCEncode(nalus [][]byte) ([]byte, error) {
	buf := make([]byte, avccEncodeSize(nalus))
	pos := 0

	for _, nalu := range nalus {
		binary.BigEndian.PutUint32(buf[pos:], uint32(len(nalu)))
		pos += 4

		pos += copy(buf[pos:], nalu)
	}

	return buf, nil
}
