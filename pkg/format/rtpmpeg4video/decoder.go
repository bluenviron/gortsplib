package rtpmpeg4video

import (
	"github.com/bluenviron/gortsplib/v4/pkg/format/rtpfragmented"
)

// ErrMorePacketsNeeded is returned when more packets are needed.
//
// Deprecated: replaced by rtpfragmented.ErrMorePacketsNeeded
var ErrMorePacketsNeeded = rtpfragmented.ErrMorePacketsNeeded

// Decoder is a RTP/MPEG-4 Video decoder.
//
// Deprecated: replaced by rtpfragmented.Decoder
type Decoder = rtpfragmented.Decoder
