package h264

// AntiCompetitionRemove removes the anti-competition bytes from a NALU.
func AntiCompetitionRemove(nalu []byte) []byte {
	// 0x00 0x00 0x03 0x00 -> 0x00 0x00 0x00
	// 0x00 0x00 0x03 0x01 -> 0x00 0x00 0x01
	// 0x00 0x00 0x03 0x02 -> 0x00 0x00 0x02
	// 0x00 0x00 0x03 0x03 -> 0x00 0x00 0x03

	n := 0
	step := 0
	start := 0

	for i, b := range nalu {
		switch step {
		case 0:
			if b == 0 {
				step++
			}

		case 1:
			if b == 0 {
				step++
			} else {
				step = 0
			}

		case 2:
			if b == 3 {
				step++
			} else {
				step = 0
			}

		case 3:
			switch b {
			case 3, 2, 1, 0:
				n += len(nalu[start : i-3])
				n += 3
				step = 0
				start = i + 1

			default:
				step = 0
			}
		}
	}

	n += len(nalu[start:])

	ret := make([]byte, n)
	n = 0
	step = 0
	start = 0

	for i, b := range nalu {
		switch step {
		case 0:
			if b == 0 {
				step++
			}

		case 1:
			if b == 0 {
				step++
			} else {
				step = 0
			}

		case 2:
			if b == 3 {
				step++
			} else {
				step = 0
			}

		case 3:
			switch b {
			case 3, 2, 1, 0:
				n += copy(ret[n:], nalu[start:i-3])
				n += copy(ret[n:], []byte{0x00, 0x00, b})
				step = 0
				start = i + 1

			default:
				step = 0
			}
		}
	}

	copy(ret[n:], nalu[start:])

	return ret
}
