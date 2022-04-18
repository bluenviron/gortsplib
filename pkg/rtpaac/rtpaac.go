// Package rtpaac contains a RTP/AAC decoder and encoder.
package rtpaac

const (
	rtpVersion = 0x02

	// i've never seen a 5kbit AU, but anyway....
	maxAUSize = 5 * 1024
)
