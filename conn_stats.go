package gortsplib

// ConnStats are connection statistics.
type ConnStats struct {
	// inbound bytes
	InboundBytes uint64
	// outbound bytes
	OutboundBytes uint64

	// Deprecated: use InboundBytes.
	BytesReceived uint64
	// Deprecated: use OutboundBytes.
	BytesSent uint64
}
