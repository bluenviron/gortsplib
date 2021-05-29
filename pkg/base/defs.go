package base

// StreamProtocol is the protocol of a stream.
type StreamProtocol int

const (
	// StreamProtocolUDP means that the stream uses the UDP protocol
	StreamProtocolUDP StreamProtocol = iota

	// StreamProtocolTCP means that the stream uses the TCP protocol
	StreamProtocolTCP
)

var streamProtocolLabels = map[StreamProtocol]string{
	StreamProtocolUDP: "UDP",
	StreamProtocolTCP: "TCP",
}

// String implements fmt.Stringer.
func (sp StreamProtocol) String() string {
	if l, ok := streamProtocolLabels[sp]; ok {
		return l
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

var streamDeliveryLabels = map[StreamDelivery]string{
	StreamDeliveryUnicast:   "unicast",
	StreamDeliveryMulticast: "multicast",
}

// String implements fmt.Stringer.
func (sc StreamDelivery) String() string {
	if l, ok := streamDeliveryLabels[sc]; ok {
		return l
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

var streamTypeLabels = map[StreamType]string{
	StreamTypeRTP:  "RTP",
	StreamTypeRTCP: "RTCP",
}

// String implements fmt.Stringer
func (st StreamType) String() string {
	if l, ok := streamTypeLabels[st]; ok {
		return l
	}
	return "unknown"
}
