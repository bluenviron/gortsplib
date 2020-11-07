package base

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
type StreamType int

const (
	// StreamTypeRtp means that the stream contains RTP packets
	StreamTypeRtp StreamType = iota

	// StreamTypeRtcp means that the stream contains RTCP packets
	StreamTypeRtcp
)

// String implements fmt.Stringer
func (st StreamType) String() string {
	switch st {
	case StreamTypeRtp:
		return "RTP"

	case StreamTypeRtcp:
		return "RTCP"
	}
	return "unknown"
}
