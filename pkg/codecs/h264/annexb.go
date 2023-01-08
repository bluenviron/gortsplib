package h264

import (
	"fmt"
)

// AnnexBUnmarshal decodes NALUs from the Annex-B stream format.
func AnnexBUnmarshal(byts []byte) ([][]byte, error) {
	bl := len(byts)
	initZeroCount := 0
	start := 0

outer:
	for {
		if start >= bl || start >= 4 {
			return nil, fmt.Errorf("initial delimiter not found")
		}

		switch initZeroCount {
		case 0, 1:
			if byts[start] != 0 {
				return nil, fmt.Errorf("initial delimiter not found")
			}
			initZeroCount++

		case 2, 3:
			switch byts[start] {
			case 1:
				start++
				break outer

			case 0:

			default:
				return nil, fmt.Errorf("initial delimiter not found")
			}
			initZeroCount++
		}

		start++
	}

	zeroCount := 0
	n := 0

	for i := start; i < bl; i++ {
		switch byts[i] {
		case 0:
			zeroCount++

		case 1:
			if zeroCount == 2 || zeroCount == 3 {
				n++
			}
			zeroCount = 0

		default:
			zeroCount = 0
		}
	}

	if (n + 1) > MaxNALUsPerGroup {
		return nil, fmt.Errorf("NALU count (%d) exceeds maximum allowed (%d)",
			n+1, MaxNALUsPerGroup)
	}

	ret := make([][]byte, n+1)
	pos := 0
	start = initZeroCount + 1
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
				l := delimStart - start
				if l == 0 {
					return nil, fmt.Errorf("invalid NALU")
				}
				if l > MaxNALUSize {
					return nil, fmt.Errorf("NALU size (%d) is too big (maximum is %d)", l, MaxNALUSize)
				}

				ret[pos] = byts[start:delimStart]
				pos++
				start = i + 1
			}
			zeroCount = 0

		default:
			zeroCount = 0
		}
	}

	l := bl - start
	if l == 0 {
		return nil, fmt.Errorf("invalid NALU")
	}
	if l > MaxNALUSize {
		return nil, fmt.Errorf("NALU size (%d) is too big (maximum is %d)", l, MaxNALUSize)
	}

	ret[pos] = byts[start:bl]

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
