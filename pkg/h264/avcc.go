package h264

import (
	"encoding/binary"
	"fmt"
)

// AVCCDecode decodes NALUs from the AVCC stream format.
func AVCCDecode(buf []byte) ([][]byte, error) {
	bl := len(buf)
	pos := 0
	var ret [][]byte

	for {
		if (bl - pos) < 4 {
			return nil, fmt.Errorf("invalid length")
		}

		le := int(binary.BigEndian.Uint32(buf[pos:]))
		pos += 4

		if (bl - pos) < le {
			return nil, fmt.Errorf("invalid length")
		}

		if (bl - pos) > maxNALUSize {
			return nil, fmt.Errorf("NALU size (%d) is too big (maximum is %d)", bl-pos, maxNALUSize)
		}

		ret = append(ret, buf[pos:pos+le])
		pos += le

		if (bl - pos) == 0 {
			break
		}
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
