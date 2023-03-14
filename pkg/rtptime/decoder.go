// Package rtptime contains a RTP timestamp decoder and encoder.
package rtptime

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

// NewDecoder allocates a Decoder.
func NewDecoder(clockRate int) *Decoder {
	return &Decoder{
		clockRate: time.Duration(clockRate),
	}
}

// Decode decodes a timestamp.
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

	// avoid an int64 overflow and preserve resolution by splitting division into two parts:
	// first add the integer part, then the decimal part.
	secs := d.overall / d.clockRate
	dec := d.overall % d.clockRate
	return secs*time.Second + dec*time.Second/d.clockRate
}
