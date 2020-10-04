package base

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
