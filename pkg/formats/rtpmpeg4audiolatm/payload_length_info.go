package rtpmpeg4audiolatm

import (
	"fmt"
)

func payloadLengthInfoDecode(buf []byte) (int, int, error) {
	lb := len(buf)
	l := 0
	n := 0

	for {
		if (lb - n) == 0 {
			return 0, 0, fmt.Errorf("not enough bytes")
		}

		b := buf[n]
		n++
		l += int(b)

		if b != 0xFF {
			break
		}
	}

	return l, n, nil
}

func payloadLengthInfoEncodeSize(auLen int) int {
	return auLen/255 + 1
}

func payloadLengthInfoEncode(plil int, auLen int, buf []byte) {
	for i := 0; i < plil; i++ {
		buf[i] = 255
	}
	buf[plil-1] = byte(auLen % 255)
}
