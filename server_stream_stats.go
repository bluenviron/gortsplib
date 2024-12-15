package gortsplib

// ServerStreamStats are ServerStream statistics.
type ServerStreamStats struct {
	// sent bytes
	BytesSent uint64
	// number of sent RTP packets
	RTPPacketsSent uint64
	// number of sent RTCP packets
	RTCPPacketsSent uint64
}
