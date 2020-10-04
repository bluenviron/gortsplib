package gortsplib

import (
	"github.com/aler9/gortsplib/base"
	"github.com/aler9/gortsplib/headers"
)

// StreamProtocol is the protocol of a stream.
type StreamProtocol = headers.StreamProtocol

const (
	// StreamProtocolUDP means that the stream uses the UDP protocol
	StreamProtocolUDP = headers.StreamProtocolUDP

	// StreamProtocolTCP means that the stream uses the TCP protocol
	StreamProtocolTCP = headers.StreamProtocolTCP
)

// StreamCast is the cast method of a stream.
type StreamCast = headers.StreamCast

const (
	// StreamUnicast means that the stream is unicasted
	StreamUnicast = headers.StreamUnicast

	// StreamMulticast means that the stream is multicasted
	StreamMulticast = headers.StreamMulticast
)

// StreamType is the stream type.
type StreamType = base.StreamType

const (
	// StreamTypeRtp means that the stream contains RTP packets
	StreamTypeRtp = base.StreamTypeRtp

	// StreamTypeRtcp means that the stream contains RTCP packets
	StreamTypeRtcp = base.StreamTypeRtcp
)

// SetupMode is the setup mode.
type SetupMode int

const (
	// SetupModePlay is the "play" setup mode
	SetupModePlay SetupMode = iota

	// SetupModeRecord is the "record" setup mode
	SetupModeRecord
)

// String implements fmt.Stringer.
func (sm SetupMode) String() string {
	switch sm {
	case SetupModePlay:
		return "play"

	case SetupModeRecord:
		return "record"
	}
	return "unknown"
}
