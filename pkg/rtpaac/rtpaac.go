// Package rtpaac contains a RTP/AAC decoder and encoder.
package rtpaac

import (
	"time"
)

// AUAndTimestamp is an Access Unit and its timestamp.
type AUAndTimestamp struct {
	Timestamp time.Duration
	AU        []byte
}
