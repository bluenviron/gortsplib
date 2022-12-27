package h264

// IDRPresent check if there's an IDR inside provided NALUs.
func IDRPresent(nalus [][]byte) bool {
	for _, nalu := range nalus {
		typ := NALUType(nalu[0] & 0x1F)
		if typ == NALUTypeIDR {
			return true
		}
	}
	return false
}
