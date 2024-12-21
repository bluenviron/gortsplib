package gortsplib

// ServerSessionStats are ServerSession statistics.
type ServerSessionStats struct {
	// received bytes
	BytesReceived uint64
	// sent bytes
	BytesSent uint64
	// number of RTP packets correctly received and processed
	RTPPacketsReceived uint64
	// number of sent RTP packets
	RTPPacketsSent uint64
	// number of lost RTP packets
	RTPPacketsLost uint64
	// number of RTP packets that could not be processed
	RTPPacketsInError uint64
	// mean jitter of received RTP packets
	RTPJitter float64
	// number of RTCP packets correctly received and processed
	RTCPPacketsReceived uint64
	// number of sent RTCP packets
	RTCPPacketsSent uint64
	// number of RTCP packets that could not be processed
	RTCPPacketsInError uint64
}
