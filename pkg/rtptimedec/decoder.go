// Package rtptimedec contains a RTP timestamp decoder.
package rtptimedec

import (
	"time"
)

const negativeThreshold = 0xFFFFFFFF / 2

// Decoder is a RTP timestamp decoder.
type Decoder struct {
	clockRate   time.Duration
	initialized bool
	overall     time.Duration
	prev        uint32
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
		d.prev = ts
		return 0
	}

	diff := ts - d.prev

	// negative difference
	if diff > negativeThreshold {
		diff = d.prev - ts
		d.prev = ts
		d.overall -= time.Duration(diff)
	} else {
		d.prev = ts
		d.overall += time.Duration(diff)
	}

	// avoid an int64 overflow and keep resolution by splitting division into two parts:
	// first add the integer part, then the decimal part.
	secs := d.overall / d.clockRate
	dec := d.overall % d.clockRate
	return secs*time.Second + dec*time.Second/d.clockRate
}
