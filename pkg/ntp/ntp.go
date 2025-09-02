// Package ntp contains functions to encode and decode timestamps to/from NTP format.
package ntp

import (
	"math"
	"time"
)

// Encode encodes a timestamp in NTP format.
// Specification: RFC3550, section 4
func Encode(t time.Time) uint64 {
	ntp := uint64(t.UnixNano()) + 2208988800*1000000000
	secs := ntp / 1000000000
	fractional := uint64(math.Round(float64((ntp%1000000000)*(1<<32)) / 1000000000))
	return secs<<32 | fractional
}

// Decode decodes a timestamp from NTP format.
// Specification: RFC3550, section 4
func Decode(v uint64) time.Time {
	secs := int64((v >> 32) - 2208988800)
	nanos := int64(math.Round(float64(((v & 0xFFFFFFFF) * 1000000000) / (1 << 32))))
	return time.Unix(secs, nanos)
}
