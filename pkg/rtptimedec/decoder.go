// Package rtptimedec contains a RTP timestamp decoder.
package rtptimedec

import (
	"time"
)

// Decoder is a RTP timestamp decoder.
type Decoder struct {
	clockRate time.Duration
	tsAdd     int64
	tsInitial *int64
	tsPrev    *int64
}

// New allocates a Decoder.
func New(clockRate int) *Decoder {
	return &Decoder{
		clockRate: time.Duration(clockRate),
	}
}

// Decode decodes a RTP timestamp.
func (d *Decoder) Decode(ts uint32) time.Duration {
	ts64 := int64(ts)

	if d.tsPrev != nil {
		diff := ts64 - *d.tsPrev
		switch {
		case diff < -0xFFFFFF: // overflow
			d.tsAdd += 0x100000000

		case diff > 0xFFFFFF: // timestamp overflowed then went back
			d.tsAdd -= 0x100000000
		}
	}

	d.tsPrev = &ts64

	if d.tsInitial == nil {
		d.tsInitial = &ts64
	}

	return time.Duration(ts64+d.tsAdd-*d.tsInitial) * time.Second / d.clockRate
}
