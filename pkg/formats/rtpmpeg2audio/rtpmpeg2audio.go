// Package rtpmpeg2audio contains a RTP/MPEG-1/2 Audio decoder and encoder.
package rtpmpeg2audio

import (
	"github.com/bluenviron/gortsplib/v3/pkg/formats/rtpmpeg1audio"
)

// ErrMorePacketsNeeded is an alis for rtpmpeg1audio.ErrMorePacketsNeeded.
//
// Deprecated: replaced by rtpmpeg1audio.ErrMorePacketsNeeded.
var ErrMorePacketsNeeded = rtpmpeg1audio.ErrMorePacketsNeeded

// ErrNonStartingPacketAndNoPrevious is an alis for rtpmpeg1audio.ErrNonStartingPacketAndNoPrevious.
//
// Deprecated: replaced by rtpmpeg1audio.ErrNonStartingPacketAndNoPrevious.
var ErrNonStartingPacketAndNoPrevious = rtpmpeg1audio.ErrNonStartingPacketAndNoPrevious

// Decoder is an alis for rtpmpeg1audio.Decoder.
//
// Deprecated: replaced by rtpmpeg1audio.Decoder.
type Decoder = rtpmpeg1audio.Decoder

// Encoder is an alis for rtpmpeg1audio.Encoder.
//
// Deprecated: replaced by rtpmpeg1audio.Encoder.
type Encoder = rtpmpeg1audio.Encoder
