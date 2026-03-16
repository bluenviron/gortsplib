package gortsplib

import (
	"github.com/bluenviron/gortsplib/v5/pkg/description"
	"github.com/bluenviron/gortsplib/v5/pkg/format"
)

// ServerStreamStatsFormat are stream format statistics.
type ServerStreamStatsFormat struct {
	// outbound RTP packets
	OutboundRTPPackets uint64
	// local SSRC
	LocalSSRC uint32

	// Deprecated: use OutboundRTPPackets.
	RTPPacketsSent uint64
}

// ServerStreamStatsMedia are stream media statistics.
type ServerStreamStatsMedia struct {
	// outbound bytes
	OutboundBytes uint64
	// outbound RTCP packets
	OutboundRTCPPackets uint64

	// format statistics
	Formats map[format.Format]ServerStreamStatsFormat

	// Deprecated: use OutboundBytes.
	BytesSent uint64
	// Deprecated: use OutboundRTCPPackets.
	RTCPPacketsSent uint64
}

// ServerStreamStats are stream statistics.
type ServerStreamStats struct {
	// outbound bytes
	OutboundBytes uint64
	// outbound RTP packets
	OutboundRTPPackets uint64
	// outbound RTCP packets
	OutboundRTCPPackets uint64

	// media statistics
	Medias map[*description.Media]ServerStreamStatsMedia

	// Deprecated: use OutboundBytes.
	BytesSent uint64
	// Deprecated: use OutboundRTPPackets.
	RTPPacketsSent uint64
	// Deprecated: use OutboundRTCPPackets.
	RTCPPacketsSent uint64
}
