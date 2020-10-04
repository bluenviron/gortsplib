package gortsplib

import (
	"github.com/aler9/gortsplib/base"
)

// StreamProtocol is the protocol of a stream.
type StreamProtocol int

const (
	// StreamProtocolUDP means that the stream uses the UDP protocol
	StreamProtocolUDP StreamProtocol = iota

	// StreamProtocolTCP means that the stream uses the TCP protocol
	StreamProtocolTCP
)

// String implements fmt.Stringer.
func (sp StreamProtocol) String() string {
	switch sp {
	case StreamProtocolUDP:
		return "udp"

	case StreamProtocolTCP:
		return "tcp"
	}
	return "unknown"
}

// StreamCast is the cast method of a stream.
type StreamCast int

const (
	// StreamUnicast means that the stream is unicasted
	StreamUnicast StreamCast = iota

	// StreamMulticast means that the stream is multicasted
	StreamMulticast
)

// String implements fmt.Stringer.
func (sc StreamCast) String() string {
	switch sc {
	case StreamUnicast:
		return "unicast"

	case StreamMulticast:
		return "multicast"
	}
	return "unknown"
}

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
