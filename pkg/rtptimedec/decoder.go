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
	ts64 := int64(ts) + d.tsAdd

	if d.tsPrev != nil && (ts64-*d.tsPrev) < -0xFFFF {
		ts64 += 0xFFFFFFFF
		d.tsAdd += 0xFFFFFFFF
	}
	d.tsPrev = &ts64

	if d.tsInitial == nil {
		d.tsInitial = &ts64
	}

	return time.Duration(ts64-*d.tsInitial) * time.Second / d.clockRate
}
