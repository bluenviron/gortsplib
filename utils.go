package gortsplib

import (
	"github.com/aler9/gortsplib/base"
	"github.com/aler9/gortsplib/headers"
)

// StreamProtocol is the protocol of a stream.
type StreamProtocol = headers.StreamProtocol

const (
	// StreamProtocolUDP means that the stream uses the UDP protocol
	StreamProtocolUDP StreamProtocol = headers.StreamProtocolUDP

	// StreamProtocolTCP means that the stream uses the TCP protocol
	StreamProtocolTCP StreamProtocol = headers.StreamProtocolTCP
)

// StreamCast is the cast method of a stream.
type StreamCast = headers.StreamCast

const (
	// StreamUnicast means that the stream is unicasted
	StreamUnicast StreamCast = headers.StreamUnicast

	// StreamMulticast means that the stream is multicasted
	StreamMulticast StreamCast = headers.StreamMulticast
)

// StreamType is the stream type.
type StreamType = base.StreamType

const (
	// StreamTypeRtp means that the stream contains RTP packets
	StreamTypeRtp StreamType = base.StreamTypeRtp

	// StreamTypeRtcp means that the stream contains RTCP packets
	StreamTypeRtcp StreamType = base.StreamTypeRtcp
)

// TransportMode is a transport mode.
type TransportMode = headers.TransportMode

const (
	// TransportModePlay is the "play" transport mode
	TransportModePlay TransportMode = headers.TransportModePlay

	// TransportModeRecord is the "record" transport mode
	TransportModeRecord TransportMode = headers.TransportModeRecord
)
