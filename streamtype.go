package gortsplib

// StreamType is a stream type.
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
