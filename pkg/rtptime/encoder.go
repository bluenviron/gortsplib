package rtptime

import (
	"time"
)

func divCeil(n, d uint64) uint64 {
	v := n / d
	if (n % d) != 0 {
		v++
	}
	return v
}

// Encoder is a RTP timestamp encoder.
type Encoder struct {
	clockRate        time.Duration
	initialTimestamp time.Duration
}

// NewEncoder allocates an Encoder.
func NewEncoder(clockRate int, initialTimestamp uint32) *Encoder {
	return &Encoder{
		clockRate: time.Duration(clockRate),
		// ((2^32) * 1000000000) is less than 2^63
		initialTimestamp: time.Duration(divCeil(uint64(initialTimestamp)*uint64(time.Second), uint64(clockRate))),
	}
}

// Encode encodes a timestamp.
func (e *Encoder) Encode(ts time.Duration) uint32 {
	ts = e.initialTimestamp + ts
	return uint32(multiplyAndDivide(ts, e.clockRate, time.Second))
}
