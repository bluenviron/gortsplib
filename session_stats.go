package gortsplib

import (
	"time"

	"github.com/bluenviron/gortsplib/v5/pkg/description"
	"github.com/bluenviron/gortsplib/v5/pkg/format"
)

// SessionStatsFormat are session format statistics.
type SessionStatsFormat struct {
	// inbound RTP packets correctly received and processed
	InboundRTPPackets uint64
	// lost inbound RTP packets
	InboundRTPPacketsLost uint64
	// mean jitter of inbound RTP packets
	InboundRTPPacketsJitter float64
	// last sequence number of inbound RTP packets
	InboundRTPPacketsLastSequenceNumber uint16
	// last RTP time of inbound RTP packets
	InboundRTPPacketsLastRTP uint32
	// last NTP time of inbound RTP packets
	InboundRTPPacketsLastNTP time.Time

	// outbound RTP packets
	OutboundRTPPackets uint64
	// outbound RTP packets reported as lost by the remote receiver
	OutboundRTPPacketsReportedLost uint64
	// last sequence number of outbound RTP packets
	OutboundRTPPacketsLastSequenceNumber uint16
	// last RTP time of outbound RTP packets
	OutboundRTPPacketsLastRTP uint32
	// last NTP time of outbound RTP packets
	OutboundRTPPacketsLastNTP time.Time

	// local SSRC
	LocalSSRC uint32
	// remote SSRC
	RemoteSSRC uint32

	// Deprecated: use InboundRTPPacketsLastSequenceNumber or OutboundRTPPacketsLastSequenceNumber.
	RTPPacketsLastSequenceNumber uint16
	// Deprecated: use InboundRTPPacketsLastRTP or OutboundRTPPacketsLastRTP.
	RTPPacketsLastRTP uint32
	// Deprecated: use InboundRTPPacketsLastNTP or OutboundRTPPacketsLastNTP.
	RTPPacketsLastNTP time.Time
	// Deprecated: use InboundRTPPackets.
	RTPPacketsReceived uint64
	// Deprecated: use OutboundRTPPackets.
	RTPPacketsSent uint64
	// Deprecated: use OutboundRTPPacketsReportedLost.
	RTPPacketsReportedLost uint64
	// Deprecated: use InboundRTPPacketsLost.
	RTPPacketsLost uint64
	// Deprecated: use InboundRTPPacketsJitter.
	RTPPacketsJitter float64
}

// SessionStatsMedia are session media statistics.
type SessionStatsMedia struct {
	// inbound bytes
	InboundBytes uint64
	// inbound RTP packets that could not be processed
	InboundRTPPacketsInError uint64
	// inbound RTCP packets correctly received and processed
	InboundRTCPPackets uint64
	// inbound RTCP packets that could not be processed
	InboundRTCPPacketsInError uint64

	// outbound bytes
	OutboundBytes uint64
	// outbound RTCP packets
	OutboundRTCPPackets uint64

	// format statistics
	Formats map[format.Format]SessionStatsFormat

	// Deprecated: use InboundBytes.
	BytesReceived uint64
	// Deprecated: use OutboundBytes.
	BytesSent uint64
	// Deprecated: use InboundRTPPacketsInError.
	RTPPacketsInError uint64
	// Deprecated: use InboundRTCPPackets.
	RTCPPacketsReceived uint64
	// Deprecated: use OutboundRTCPPackets.
	RTCPPacketsSent uint64
	// Deprecated: use InboundRTCPPacketsInError.
	RTCPPacketsInError uint64
}

// SessionStats are session statistics.
type SessionStats struct {
	// inbound bytes
	InboundBytes uint64
	// inbound RTP packets correctly received and processed
	InboundRTPPackets uint64
	// lost inbound RTP packets
	InboundRTPPacketsLost uint64
	// inbound RTP packets that could not be processed
	InboundRTPPacketsInError uint64
	// mean jitter of inbound RTP packets
	InboundRTPPacketsJitter float64
	// inbound RTCP packets correctly received and processed
	InboundRTCPPackets uint64
	// inbound RTCP packets that could not be processed
	InboundRTCPPacketsInError uint64

	// outbound bytes
	OutboundBytes uint64
	// outbound RTP packets
	OutboundRTPPackets uint64
	// outbound RTP packets reported as lost by the remote receiver
	OutboundRTPPacketsReportedLost uint64
	// outbound RTCP packets
	OutboundRTCPPackets uint64

	// media statistics
	Medias map[*description.Media]SessionStatsMedia

	// Deprecated: use InboundBytes.
	BytesReceived uint64
	// Deprecated: use OutboundBytes.
	BytesSent uint64
	// Deprecated: use InboundRTPPackets.
	RTPPacketsReceived uint64
	// Deprecated: use OutboundRTPPackets.
	RTPPacketsSent uint64
	// Deprecated: use OutboundRTPPacketsReportedLost.
	RTPPacketsReportedLost uint64
	// Deprecated: use InboundRTPPacketsLost.
	RTPPacketsLost uint64
	// Deprecated: use InboundRTPPacketsInError.
	RTPPacketsInError uint64
	// Deprecated: use InboundRTPPacketsJitter.
	RTPPacketsJitter float64
	// Deprecated: use InboundRTCPPackets.
	RTCPPacketsReceived uint64
	// Deprecated: use OutboundRTCPPackets.
	RTCPPacketsSent uint64
	// Deprecated: use InboundRTCPPacketsInError.
	RTCPPacketsInError uint64
}

func sessionStatsFromMedias(mediaStats map[*description.Media]SessionStatsMedia) SessionStats {
	inboundBytes := uint64(0)
	outboundBytes := uint64(0)
	inboundRTPPackets := uint64(0)
	outboundRTPPackets := uint64(0)
	outboundRTPPacketsReportedLost := uint64(0)
	inboundRTPPacketsLost := uint64(0)
	inboundRTPPacketsInError := uint64(0)
	inboundRTPPacketsJitter := float64(0)
	inboundRTPPacketsJitterCount := float64(0)
	inboundRTCPPackets := uint64(0)
	outboundRTCPPackets := uint64(0)
	inboundRTCPPacketsInError := uint64(0)

	for _, ms := range mediaStats {
		inboundBytes += ms.InboundBytes
		outboundBytes += ms.OutboundBytes
		inboundRTPPacketsInError += ms.InboundRTPPacketsInError
		inboundRTCPPackets += ms.InboundRTCPPackets
		outboundRTCPPackets += ms.OutboundRTCPPackets
		inboundRTCPPacketsInError += ms.InboundRTCPPacketsInError

		for _, fs := range ms.Formats {
			inboundRTPPackets += fs.InboundRTPPackets
			outboundRTPPackets += fs.OutboundRTPPackets
			outboundRTPPacketsReportedLost += fs.OutboundRTPPacketsReportedLost
			inboundRTPPacketsLost += fs.InboundRTPPacketsLost
			inboundRTPPacketsJitter += fs.InboundRTPPacketsJitter
			inboundRTPPacketsJitterCount++
		}
	}

	if inboundRTPPacketsJitterCount != 0 {
		inboundRTPPacketsJitter /= inboundRTPPacketsJitterCount
	}

	return SessionStats{
		InboundBytes:                   inboundBytes,
		InboundRTPPackets:              inboundRTPPackets,
		InboundRTPPacketsLost:          inboundRTPPacketsLost,
		InboundRTPPacketsInError:       inboundRTPPacketsInError,
		InboundRTPPacketsJitter:        inboundRTPPacketsJitter,
		InboundRTCPPackets:             inboundRTCPPackets,
		InboundRTCPPacketsInError:      inboundRTCPPacketsInError,
		OutboundBytes:                  outboundBytes,
		OutboundRTPPackets:             outboundRTPPackets,
		OutboundRTPPacketsReportedLost: outboundRTPPacketsReportedLost,
		OutboundRTCPPackets:            outboundRTCPPackets,
		Medias:                         mediaStats,
		// deprecated
		BytesReceived:          inboundBytes,
		BytesSent:              outboundBytes,
		RTPPacketsReceived:     inboundRTPPackets,
		RTPPacketsSent:         outboundRTPPackets,
		RTPPacketsReportedLost: outboundRTPPacketsReportedLost,
		RTPPacketsLost:         inboundRTPPacketsLost,
		RTPPacketsInError:      inboundRTPPacketsInError,
		RTPPacketsJitter:       inboundRTPPacketsJitter,
		RTCPPacketsReceived:    inboundRTCPPackets,
		RTCPPacketsSent:        outboundRTCPPackets,
		RTCPPacketsInError:     inboundRTCPPacketsInError,
	}
}
