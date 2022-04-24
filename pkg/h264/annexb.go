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

// AnnexBEncode encodes NALUs into the Annex-B stream format.
func AnnexBEncode(nalus [][]byte) ([]byte, error) {
	var ret []byte

	for _, nalu := range nalus {
		ret = append(ret, []byte{0x00, 0x00, 0x00, 0x01}...)
		ret = append(ret, nalu...)
	}

	return ret, nil
}
