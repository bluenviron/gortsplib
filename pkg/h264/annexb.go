package h264

import (
	"fmt"
)

// AnnexBDecode decodes NALUs from the Annex-B stream format.
func AnnexBDecode(byts []byte) ([][]byte, error) {
	bl := len(byts)
	zeroCount := 0

outer:
	for i := 0; i < bl; i++ {
		switch byts[i] {
		case 0:
			zeroCount++

		case 1:
			break outer

		default:
			return nil, fmt.Errorf("unexpected byte: %d", byts[i])
		}
	}
	if zeroCount != 2 && zeroCount != 3 {
		return nil, fmt.Errorf("initial delimiter not found")
	}

	var ret [][]byte
	start := zeroCount + 1
	zeroCount = 0
	delimStart := 0

	for i := start; i < bl; i++ {
		switch byts[i] {
		case 0:
			if zeroCount == 0 {
				delimStart = i
			}
			zeroCount++

		case 1:
			if zeroCount == 2 || zeroCount == 3 {
				nalu := byts[start:delimStart]
				if len(nalu) == 0 {
					return nil, fmt.Errorf("empty NALU")
				}

				ret = append(ret, nalu)
				start = i + 1
			}
			zeroCount = 0

		default:
			zeroCount = 0
		}
	}

	nalu := byts[start:bl]
	if len(nalu) == 0 {
		return nil, fmt.Errorf("empty NALU")
	}
	ret = append(ret, nalu)

	return ret, nil
}

func annexBEncodeSize(nalus [][]byte) int {
	n := 0
	for _, nalu := range nalus {
		n += 4 + len(nalu)
	}
	return n
}

// AnnexBEncode encodes NALUs into the Annex-B stream format.
func AnnexBEncode(nalus [][]byte) ([]byte, error) {
	buf := make([]byte, annexBEncodeSize(nalus))
	pos := 0

	for _, nalu := range nalus {
		pos += copy(buf[pos:], []byte{0x00, 0x00, 0x00, 0x01})
		pos += copy(buf[pos:], nalu)
	}

	return buf, nil
}
