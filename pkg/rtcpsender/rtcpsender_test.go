package rtcpsender

import (
	"testing"
	"time"

	"github.com/pion/rtcp"
	"github.com/pion/rtp"
	"github.com/stretchr/testify/require"
)

func TestRTCPSender(t *testing.T) {
	rs := New(90000)

	require.Equal(t, nil, rs.Report(time.Now()))

	rtpPkt := rtp.Packet{
		Header: rtp.Header{
			Version:        2,
			Marker:         true,
			PayloadType:    96,
			SequenceNumber: 946,
			Timestamp:      1287987768,
			SSRC:           0xba9da416,
		},
		Payload: []byte("\x00\x00"),
	}
	ts := time.Date(2008, 0o5, 20, 22, 15, 20, 0, time.UTC)
	rs.ProcessPacketRTP(ts, &rtpPkt)

	rtpPkt = rtp.Packet{
		Header: rtp.Header{
			Version:        2,
			Marker:         true,
			PayloadType:    96,
			SequenceNumber: 947,
			Timestamp:      1287987768 + 45000,
			SSRC:           0xba9da416,
		},
		Payload: []byte("\x00\x00"),
	}
	ts = time.Date(2008, 0o5, 20, 22, 15, 20, 500000000, time.UTC)
	rs.ProcessPacketRTP(ts, &rtpPkt)

	expectedPkt := rtcp.SenderReport{
		SSRC:        0xba9da416,
		NTPTime:     0xcbddcc34999997ff,
		RTPTime:     0x4d185ae8,
		PacketCount: 2,
		OctetCount:  4,
	}
	ts = time.Date(2008, 0o5, 20, 22, 16, 20, 600000000, time.UTC)
	require.Equal(t, &expectedPkt, rs.Report(ts))
}
