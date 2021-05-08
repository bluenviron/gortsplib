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
		return "UDP"

	case StreamProtocolTCP:
		return "TCP"
	}
	return "unknown"
}

// StreamDelivery is the delivery method of a stream.
type StreamDelivery int

const (
	// StreamDeliveryUnicast means that the stream is unicasted
	StreamDeliveryUnicast StreamDelivery = iota

	// StreamDeliveryMulticast means that the stream is multicasted
	StreamDeliveryMulticast
)

// String implements fmt.Stringer.
func (sc StreamDelivery) String() string {
	switch sc {
	case StreamDeliveryUnicast:
		return "unicast"

	case StreamDeliveryMulticast:
		return "multicast"
	}
	return "unknown"
}

// StreamType is the stream type.
type StreamType int

const (
	// StreamTypeRTP means that the stream contains RTP packets
	StreamTypeRTP StreamType = iota

	// StreamTypeRTCP means that the stream contains RTCP packets
	StreamTypeRTCP
)

// String implements fmt.Stringer
func (st StreamType) String() string {
	switch st {
	case StreamTypeRTP:
		return "RTP"

	case StreamTypeRTCP:
		return "RTCP"
	}
	return "unknown"
}
