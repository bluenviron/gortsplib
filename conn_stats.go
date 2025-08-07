package gortsplib

// StatsConn are connection statistics.
//
// Deprecated: renamed into ConnStats.
type StatsConn = ConnStats

// ConnStats are connection statistics.
type ConnStats struct {
	// received bytes
	BytesReceived uint64
	// sent bytes
	BytesSent uint64
}
