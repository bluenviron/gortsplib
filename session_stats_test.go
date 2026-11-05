package gortsplib

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/bluenviron/gortsplib/v5/pkg/description"
	"github.com/bluenviron/gortsplib/v5/pkg/format"
)

func TestSessionStatsJitter(t *testing.T) {
	for _, ca := range []struct {
		name     string
		formats  map[format.Format]SessionStatsFormat
		jitterUs *float64
		raw      float64
	}{
		{
			name: "mixed clocks",
			formats: map[format.Format]SessionStatsFormat{
				&format.H264{}: {InboundRTPPackets: 100, InboundRTPPacketsJitter: 900},
				&format.Opus{}: {InboundRTPPackets: 1, InboundRTPPacketsJitter: 960},
			},
			jitterUs: newFloat64(15_000),
			raw:      930,
		},
		{
			name: "idle formats excluded",
			formats: map[format.Format]SessionStatsFormat{
				&format.H264{}:    {InboundRTPPackets: 1, InboundRTPPacketsJitter: 900},
				&format.Opus{}:    {},
				&format.Generic{}: {},
			},
			jitterUs: newFloat64(10_000),
			raw:      300,
		},
		{
			name: "no traffic",
			formats: map[format.Format]SessionStatsFormat{
				&format.H264{}: {InboundRTPPacketsJitter: 900},
			},
			raw: 900,
		},
		{
			name: "unknown active clock",
			formats: map[format.Format]SessionStatsFormat{
				&format.H264{}:    {InboundRTPPackets: 1, InboundRTPPacketsJitter: 900},
				&format.Generic{}: {InboundRTPPackets: 1, InboundRTPPacketsJitter: 100},
			},
			raw: 500,
		},
		{
			name: "negative clock",
			formats: map[format.Format]SessionStatsFormat{
				&format.Generic{ClockRat: -1}: {InboundRTPPackets: 1},
			},
		},
		{
			name: "measured zero",
			formats: map[format.Format]SessionStatsFormat{
				&format.H264{}: {InboundRTPPackets: 1},
			},
			jitterUs: newFloat64(0),
		},
		{name: "no formats"},
	} {
		t.Run(ca.name, func(t *testing.T) {
			stats := sessionStatsFromMedias(map[*description.Media]SessionStatsMedia{
				{}: {Formats: ca.formats},
			})
			require.Equal(t, ca.jitterUs, stats.InboundRTPPacketsJitterUs)
			require.Equal(t, ca.raw, stats.InboundRTPPacketsJitter)
			require.Equal(t, ca.raw, stats.RTPPacketsJitter)
		})
	}
}

func newFloat64(v float64) *float64 {
	return &v
}

func TestSessionStatsJitterAcrossMedias(t *testing.T) {
	stats := sessionStatsFromMedias(map[*description.Media]SessionStatsMedia{
		{}: {Formats: map[format.Format]SessionStatsFormat{
			&format.H264{}: {InboundRTPPackets: 1, InboundRTPPacketsJitter: 900},
		}},
		{}: {Formats: map[format.Format]SessionStatsFormat{
			&format.Opus{}: {InboundRTPPackets: 1, InboundRTPPacketsJitter: 960},
		}},
	})
	require.NotNil(t, stats.InboundRTPPacketsJitterUs)
	require.InDelta(t, 15_000, *stats.InboundRTPPacketsJitterUs, 0.001)
}
