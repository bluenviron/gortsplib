package rtptime

import (
	"math"
	"time"
)

// Encoder is a RTP timestamp encoder.
type Encoder struct {
	clockRate        float64
	initialTimestamp time.Duration
}

// NewEncoder allocates an Encoder.
func NewEncoder(clockRate int, initialTimestamp uint32) *Encoder {
	return &Encoder{
		clockRate:        float64(clockRate),
		initialTimestamp: time.Duration(math.Ceil(float64(initialTimestamp) * float64(time.Second) / float64(clockRate))),
	}
}

// Encode encodes a timestamp.
func (e *Encoder) Encode(ts time.Duration) uint32 {
	return uint32((e.initialTimestamp + ts).Seconds() * e.clockRate)
}
