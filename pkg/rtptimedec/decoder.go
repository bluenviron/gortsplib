// Package rtptimedec contains a RTP timestamp decoder.
package rtptimedec

import (
	"time"
)

const negativeThreshold = 0xFFFFFFF

// Decoder is a RTP timestamp decoder.
type Decoder struct {
	clockRate   time.Duration
	initialized bool
	tsOverall   time.Duration
	tsPrev      uint32
}

// New allocates a Decoder.
func New(clockRate int) *Decoder {
	return &Decoder{
		clockRate: time.Duration(clockRate),
	}
}

// Decode decodes a RTP timestamp.
func (d *Decoder) Decode(ts uint32) time.Duration {
	if !d.initialized {
		d.initialized = true
		d.tsPrev = ts
		return 0
	}

	diff := ts - d.tsPrev

	// negative difference
	if diff > negativeThreshold {
		diff = d.tsPrev - ts
		d.tsPrev = ts
		d.tsOverall -= time.Duration(diff)
	} else {
		d.tsPrev = ts
		d.tsOverall += time.Duration(diff)
	}

	// avoid an int64 overflow and preserve resolution by splitting division into two parts:
	// first add the integer part, then the decimal part.
	secs := d.tsOverall / d.clockRate
	dec := d.tsOverall % d.clockRate
	return secs*time.Second + dec*time.Second/d.clockRate
}
