package h264

import (
	"bytes"
	"fmt"
)

// AnnexBUnmarshal decodes NALUs from the Annex-B stream format.
func AnnexBUnmarshal(byts []byte) ([][]byte, error) {
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

	start := zeroCount + 1

	var n int
	if zeroCount == 2 {
		n = bytes.Count(byts[start:], []byte{0x00, 0x00, 0x01})
	} else {
		n = bytes.Count(byts[start:], []byte{0x00, 0x00, 0x00, 0x01})
	}

	ret := make([][]byte, n+1)
	pos := 0

	curZeroCount := 0
	delimStart := 0

	for i := start; i < bl; i++ {
		switch byts[i] {
		case 0:
			if curZeroCount == 0 {
				delimStart = i
			}
			curZeroCount++

		case 1:
			if curZeroCount == zeroCount {
				if (delimStart - start) > MaxNALUSize {
					return nil, fmt.Errorf("NALU size (%d) is too big (maximum is %d)", delimStart-start, MaxNALUSize)
				}

				nalu := byts[start:delimStart]
				if len(nalu) == 0 {
					return nil, fmt.Errorf("empty NALU")
				}

				ret[pos] = nalu
				pos++
				start = i + 1
			}
			curZeroCount = 0

		default:
			curZeroCount = 0
		}
	}

	if (bl - start) > MaxNALUSize {
		return nil, fmt.Errorf("NALU size (%d) is too big (maximum is %d)", bl-start, MaxNALUSize)
	}

	nalu := byts[start:bl]
	if len(nalu) == 0 {
		return nil, fmt.Errorf("empty NALU")
	}
	ret[pos] = nalu

	return ret, nil
}

func annexBMarshalSize(nalus [][]byte) int {
	n := 0
	for _, nalu := range nalus {
		n += 4 + len(nalu)
	}
	return n
}

// AnnexBMarshal encodes NALUs into the Annex-B stream format.
func AnnexBMarshal(nalus [][]byte) ([]byte, error) {
	buf := make([]byte, annexBMarshalSize(nalus))
	pos := 0

	for _, nalu := range nalus {
		pos += copy(buf[pos:], []byte{0x00, 0x00, 0x00, 0x01})
		pos += copy(buf[pos:], nalu)
	}

	return buf, nil
}
