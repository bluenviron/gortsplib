// Package rtph264 contains a RTP/H264 decoder and encoder.
package rtph264

import (
	"time"
)

// NALUAndTimestamp is a Network Abstraction Layer Unit and its timestamp.
type NALUAndTimestamp struct {
	Timestamp time.Duration
	NALU      []byte
}
