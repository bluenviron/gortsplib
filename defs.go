package gortsplib

import (
	"github.com/aler9/gortsplib/pkg/base"
)

// StreamType is the stream type.
type StreamType = base.StreamType

const (
	// StreamTypeRTP means that the stream contains RTP packets
	StreamTypeRTP StreamType = base.StreamTypeRTP

	// StreamTypeRTCP means that the stream contains RTCP packets
	StreamTypeRTCP StreamType = base.StreamTypeRTCP
)
