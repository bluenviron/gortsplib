package gortsplib

import (
	"github.com/aler9/gortsplib/pkg/base"
)

// StreamProtocol is the protocol of a stream.
type StreamProtocol = base.StreamProtocol

const (
	// StreamProtocolUDP means that the stream uses the UDP protocol
	StreamProtocolUDP StreamProtocol = base.StreamProtocolUDP

	// StreamProtocolTCP means that the stream uses the TCP protocol
	StreamProtocolTCP StreamProtocol = base.StreamProtocolTCP
)

// StreamType is the stream type.
type StreamType = base.StreamType

const (
	// StreamTypeRTP means that the stream contains RTP packets
	StreamTypeRTP StreamType = base.StreamTypeRTP

	// StreamTypeRTCP means that the stream contains RTCP packets
	StreamTypeRTCP StreamType = base.StreamTypeRTCP
)
