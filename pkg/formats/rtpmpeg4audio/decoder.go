package rtpmpeg4audio

import (
	"errors"

	"github.com/bluenviron/gortsplib/v3/pkg/formats/rtpmpeg4audiogeneric"
)

// ErrMorePacketsNeeded is an alis for rtpmpeg4audiogeneric.ErrMorePacketsNeeded.
var ErrMorePacketsNeeded = errors.New("need more packets")

// Decoder is an alias for rtpmpeg4audiogeneric.Decoder.
type Decoder = rtpmpeg4audiogeneric.Decoder
