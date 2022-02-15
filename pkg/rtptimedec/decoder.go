// Package rtptimedec contains a RTP timestamp decoder.
package rtptimedec

import (
	"time"
)

// Decoder is a RTP timestamp decoder.
type Decoder struct {
	clockRate time.Duration
	tsAdd     *int64
	tsPrev    int64
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

	if d.tsAdd == nil {
		d.tsPrev = ts64
		ts64 = -ts64
		d.tsAdd = &ts64
		return 0
	}

	diff := ts64 - d.tsPrev
	d.tsPrev = ts64

	switch {
	case diff < -0xFFFFFF: // overflow
		*d.tsAdd += 0x100000000

	case diff > 0xFFFFFF: // timestamp overflowed then went back
		*d.tsAdd -= 0x100000000
	}

	tot := time.Duration(ts64 + *d.tsAdd)

	// avoid an int64 overflow and preserve resolution by splitting division into two parts:
	// first add seconds, then the decimal part.
	secs := tot / d.clockRate
	dec := tot % d.clockRate
	return secs*time.Second + dec*time.Second/d.clockRate
}
